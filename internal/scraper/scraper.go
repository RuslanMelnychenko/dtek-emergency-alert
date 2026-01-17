package scraper

import (
	"dtek-emergency-alert/internal/models"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"
)

const (
	DTEK_SITE                      = "https://www.dtek-kem.com.ua/ua/shutdowns"
	modalAttempt                   = "div#modal-attention.is-open"
	closeModalButtonSelector       = "button.modal__close"
	streetInputSelector            = "input#street"
	houseInputSelector             = "input#house_num"
	streetAutocompleteListSelector = "div#streetautocomplete-list"
	houseAutocompleteListSelector  = "div#house_numautocomplete-list"
	currentOutstageCard            = "div#showCurOutage.active"
	timeFormat                     = "15:04 02.01.2006"
)

type Scraper interface {
	ScrapCurrentOutage(street, house, screenshotPath string) (*models.Outage, error)
}

type playwrightScraper struct {
	logger *log.Logger
}

func NewScraper(logger *log.Logger) Scraper {
	return &playwrightScraper{
		logger: logger,
	}
}

func (s *playwrightScraper) ScrapCurrentOutage(street, house, screenshotPath string) (*models.Outage, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("could not start playwright: %w", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
		SlowMo:   playwright.Float(100),
	})
	if err != nil {
		return nil, fmt.Errorf("could not launch browser: %w", err)
	}
	defer browser.Close()

	context, err := browser.NewContext(playwright.BrowserNewContextOptions{
		UserAgent:  playwright.String("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"),
		Locale:     playwright.String("uk-UA,uk;"),
		TimezoneId: playwright.String("Europe/Kiev"),
	})
	if err != nil {
		return nil, fmt.Errorf("could not create new context: %w", err)
	}
	defer context.Close()

	page, err := context.NewPage()
	if err != nil {
		return nil, fmt.Errorf("could not create page: %w", err)
	}

	_, err = page.Goto(DTEK_SITE, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	})
	if err != nil {
		return nil, fmt.Errorf("could not goto: %w", err)
	}
	defer page.Close()

	modalAttemptLocator := page.Locator(modalAttempt)
	if count, _ := modalAttemptLocator.Count(); count > 0 {
		modalAttemptLocator.Locator(closeModalButtonSelector).Click()
	}

	var result *models.Outage

	var wg sync.WaitGroup
	page.Once("response", func(response playwright.Response) {
		if response.URL() != "https://www.dtek-kem.com.ua/ua/ajax" {
			return
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			type statDTO struct {
				StartDate string `json:"start_date"`
				EndDate   string `json:"end_date"`
				SubType   string `json:"sub_type"`
				Type      string `json:"type"`
			}
			type respDTO struct {
				UpdateTimestamp    string `json:"updateTimestamp"`
				ShowCurOutageParam bool   `json:"showCurOutageParam"`
				Data               map[string]statDTO
			}

			var parsedResponse respDTO
			jsonErr := response.JSON(&parsedResponse)
			if jsonErr != nil {
				s.logger.Println("could not parse response: ", jsonErr)
				return
			}

			updateTimestamp, _ := time.Parse(timeFormat, parsedResponse.UpdateTimestamp)

			result = &models.Outage{
				ShowCurOutage:   parsedResponse.ShowCurOutageParam,
				UpdateTimestamp: updateTimestamp,
			}

			if result.ShowCurOutage {
				if stat, ok := parsedResponse.Data[house]; ok {
					result.StartDate, _ = time.Parse(timeFormat, stat.StartDate)
					result.EndDate, _ = time.Parse(timeFormat, stat.EndDate)
					result.Text = stat.SubType
					result.Type = stat.Type
				} else {
					s.logger.Printf("House %s not found in response data", house)
				}
			}
		}()
	})

	err = page.Fill(streetInputSelector, street)
	if err != nil {
		return nil, fmt.Errorf("could not fill street: %w", err)
	}

	streetOption := page.Locator(streetAutocompleteListSelector + " > div").Filter(playwright.LocatorFilterOptions{
		Has: page.Locator("input[value=\"" + street + "\"]"),
	}).First()
	err = streetOption.Click()
	if err != nil {
		return nil, fmt.Errorf("could not select street from dropdown: %w", err)
	}

	err = page.Locator(houseInputSelector).WaitFor(playwright.LocatorWaitForOptions{
		State: playwright.WaitForSelectorStateVisible,
	})
	if err != nil {
		return nil, fmt.Errorf("could not wait for house input to be enabled: %w", err)
	}
	_, err = page.Locator(houseInputSelector).Evaluate("element => element.disabled === false", nil)
	if err != nil {
		return nil, fmt.Errorf("house input is still disabled: %w", err)
	}

	err = page.Fill(houseInputSelector, house)
	if err != nil {
		return nil, fmt.Errorf("could not fill house: %w", err)
	}

	houseAutocompleteListLocator := page.Locator(houseAutocompleteListSelector)
	err = houseAutocompleteListLocator.WaitFor(playwright.LocatorWaitForOptions{
		State: playwright.WaitForSelectorStateVisible,
	})
	if err != nil {
		return nil, fmt.Errorf("could not wait for house autocomplete list: %w", err)
	}

	houseOption := houseAutocompleteListLocator.Locator("div").Filter(playwright.LocatorFilterOptions{
		Has: page.Locator("input[value=\"" + house + "\"]"),
	}).First()
	err = houseOption.Click()
	if err != nil {
		return nil, fmt.Errorf("could not select house from dropdown: %w", err)
	}

	card := page.Locator(currentOutstageCard)

	_, err = card.Screenshot(playwright.LocatorScreenshotOptions{
		Path: playwright.String(screenshotPath),
	})

	if err != nil {
		return nil, fmt.Errorf("could not take screenshot: %w", err)
	}

	wg.Wait()

	return result, nil
}
