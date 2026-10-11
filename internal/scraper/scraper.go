package scraper

import (
	"dtek-emergency-alert/internal/models"
	"fmt"
	"log"
	"time"

	"github.com/mxschmitt/playwright-go"
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
	dtekAjaxURL                    = "https://www.dtek-kem.com.ua/ua/ajax"
	getHomeNumTimeout              = 15 * time.Second
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

	// Defer-и виконуються у зворотному порядку: спочатку закривається сторінка
	// (незавершені response.JSON() отримують помилку), і лише потім catcher чекає
	// свої горутини та закриває канал.
	catcher := newHomeNumCatcher(house, s.logger)
	defer catcher.close()
	defer page.Close()

	if err := openShutdownsPage(page); err != nil {
		return nil, err
	}

	catcher.attach(page)

	if err := selectAddress(page, street, house); err != nil {
		return nil, err
	}

	if err := takeOutageScreenshot(page, screenshotPath); err != nil {
		return nil, err
	}

	return catcher.wait(getHomeNumTimeout)
}

// openShutdownsPage відкриває сторінку відключень і закриває модальне вікно, якщо воно є.
func openShutdownsPage(page playwright.Page) error {
	_, err := page.Goto(DTEK_SITE, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	})
	if err != nil {
		return fmt.Errorf("could not goto: %w", err)
	}

	modalAttemptLocator := page.Locator(modalAttempt)
	if count, _ := modalAttemptLocator.Count(); count > 0 {
		modalAttemptLocator.Locator(closeModalButtonSelector).Click()
	}
	return nil
}

// selectAddress вводить вулицю та будинок і обирає їх з випадаючих списків.
// Вибір вулиці ініціює запит getHomeNum, який перехоплює homeNumCatcher.
func selectAddress(page playwright.Page, street, house string) error {
	if err := selectStreet(page, street); err != nil {
		return err
	}
	return selectHouse(page, house)
}

func selectStreet(page playwright.Page, street string) error {
	if err := page.Fill(streetInputSelector, street); err != nil {
		return fmt.Errorf("could not fill street: %w", err)
	}

	streetOption := page.Locator(streetAutocompleteListSelector + " > div").Filter(playwright.LocatorFilterOptions{
		Has: page.Locator("input[value=\"" + street + "\"]"),
	}).First()
	if err := streetOption.Click(); err != nil {
		return fmt.Errorf("could not select street from dropdown: %w", err)
	}
	return nil
}

func selectHouse(page playwright.Page, house string) error {
	houseInput := page.Locator(houseInputSelector)
	err := houseInput.WaitFor(playwright.LocatorWaitForOptions{
		State: playwright.WaitForSelectorStateVisible,
	})
	if err != nil {
		return fmt.Errorf("could not wait for house input to be enabled: %w", err)
	}
	if _, err := houseInput.Evaluate("element => element.disabled === false", nil); err != nil {
		return fmt.Errorf("house input is still disabled: %w", err)
	}

	if err := page.Fill(houseInputSelector, house); err != nil {
		return fmt.Errorf("could not fill house: %w", err)
	}

	houseAutocompleteListLocator := page.Locator(houseAutocompleteListSelector)
	err = houseAutocompleteListLocator.WaitFor(playwright.LocatorWaitForOptions{
		State: playwright.WaitForSelectorStateVisible,
	})
	if err != nil {
		return fmt.Errorf("could not wait for house autocomplete list: %w", err)
	}

	houseOption := houseAutocompleteListLocator.Locator("div").Filter(playwright.LocatorFilterOptions{
		Has: page.Locator("input[value=\"" + house + "\"]"),
	}).First()
	if err := houseOption.Click(); err != nil {
		return fmt.Errorf("could not select house from dropdown: %w", err)
	}
	return nil
}

func takeOutageScreenshot(page playwright.Page, screenshotPath string) error {
	_, err := page.Locator(currentOutstageCard).Screenshot(playwright.LocatorScreenshotOptions{
		Path: playwright.String(screenshotPath),
	})
	if err != nil {
		return fmt.Errorf("could not take screenshot: %w", err)
	}
	return nil
}
