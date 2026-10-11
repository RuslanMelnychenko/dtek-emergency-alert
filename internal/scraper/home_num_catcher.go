package scraper

import (
	"dtek-emergency-alert/internal/models"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/mxschmitt/playwright-go"
)

const (
	getHomeNumMethod = "getHomeNum"
)

// homeNumCatcher перехоплює першу відповідь на запит getHomeNum і перетворює її на models.Outage.
type homeNumCatcher struct {
	house  string
	logger *log.Logger

	page       playwright.Page
	onResponse func(response playwright.Response)

	resultCh chan *models.Outage
	wg       sync.WaitGroup
	mu       sync.Mutex
	isClosed bool
}

func newHomeNumCatcher(house string, logger *log.Logger) *homeNumCatcher {
	c := &homeNumCatcher{
		house:    house,
		logger:   logger,
		resultCh: make(chan *models.Outage, 1),
	}
	// Обробник зберігаємо в полі: RemoveListener знаходить його за ідентичністю замикання.
	c.onResponse = c.handleResponse
	return c
}

// attach підписується на відповіді сторінки. Викликати до дії, що ініціює getHomeNum.
func (c *homeNumCatcher) attach(page playwright.Page) {
	c.page = page
	page.On("response", c.onResponse)
}

// wait повертає перехоплений результат або помилку, якщо відповідь не прийшла за timeout.
// Порожній результат не повертаємо: для app це виглядало б як «відключень немає».
func (c *homeNumCatcher) wait(timeout time.Duration) (*models.Outage, error) {
	select {
	case result := <-c.resultCh:
		return result, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("getHomeNum response not received within %s", timeout)
	}
}

// close чекає завершення горутин обробки і закриває канал.
// Викликати після page.Close(), щоб незавершені response.JSON() не блокували очікування.
func (c *homeNumCatcher) close() {
	c.wg.Wait()

	c.mu.Lock()
	defer c.mu.Unlock()
	c.isClosed = true
	close(c.resultCh)
}

// handleResponse викликається синхронно з диспетчера playwright,
// тому звернення до драйвера (PostData, JSON) виносимо в горутину.
func (c *homeNumCatcher) handleResponse(response playwright.Response) {
	if response.URL() != dtekAjaxURL {
		return
	}

	c.wg.Go(func() {
		// На /ua/ajax ідуть різні запити (getHomeNum, updateFact тощо), потрібен лише getHomeNum.
		if !isGetHomeNumRequest(response.Request()) {
			return
		}

		result, err := c.parseResponse(response)
		if err != nil {
			c.logger.Println("could not parse response: ", err)
			return
		}

		if c.send(result) {
			c.page.RemoveListener("response", c.onResponse)
		}
	})
}

// send не блокується і ніколи не пише в закритий канал: запізніла подія
// після виходу з ScrapCurrentOutage стає no-op замість паніки.
func (c *homeNumCatcher) send(result *models.Outage) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isClosed {
		return false
	}
	select {
	case c.resultCh <- result:
		return true
	default:
		return false
	}
}

type homeNumStatDTO struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	SubType   string `json:"sub_type"`
	Type      string `json:"type"`
}

type homeNumResponseDTO struct {
	UpdateTimestamp    string `json:"updateTimestamp"`
	ShowCurOutageParam bool   `json:"showCurOutageParam"`
	Data               map[string]homeNumStatDTO
}

func (c *homeNumCatcher) parseResponse(response playwright.Response) (*models.Outage, error) {
	var parsedResponse homeNumResponseDTO
	if err := response.JSON(&parsedResponse); err != nil {
		return nil, err
	}

	updateTimestamp, _ := time.Parse(timeFormat, parsedResponse.UpdateTimestamp)

	result := &models.Outage{
		ShowCurOutage:   parsedResponse.ShowCurOutageParam,
		UpdateTimestamp: updateTimestamp,
	}

	if !result.ShowCurOutage {
		return result, nil
	}

	stat, ok := parsedResponse.Data[c.house]
	if !ok {
		c.logger.Printf("House %s not found in response data", c.house)
		return result, nil
	}

	result.StartDate, _ = time.Parse(timeFormat, stat.StartDate)
	result.EndDate, _ = time.Parse(timeFormat, stat.EndDate)
	result.Text = stat.SubType
	result.Type = stat.Type
	return result, nil
}

func isGetHomeNumRequest(request playwright.Request) bool {
	postData, err := request.PostData()
	if err != nil {
		return false
	}
	values, err := url.ParseQuery(postData)
	if err != nil {
		return false
	}
	return values.Get("method") == getHomeNumMethod
}
