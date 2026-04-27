package main

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// StoreConfig - конфигурация магазина с токенами
type StoreConfig struct {
	CitySlug  string // slug города
	CityName  string // название города
	StoreID   int    // ID магазина
	StoreName string // название магазина
	Type      string // pickup или delivery

	// Токены для этого магазина
	SessionToken  string
	DeviceID      string
	UserSessionID string
	QratorJSID    string
}

// Предустановленные магазины С ТОКЕНАМИ
var knownStores = []StoreConfig{
	{
		// Москва
		CitySlug:      "moscow",
		CityName:      "Москва и МО",
		StoreID:       3149,
		StoreName:     "ТК124 (Мозаика)",
		Type:          "pickup",
		SessionToken:  "941104B44D8BF3FFBC2B354576C82F66",
		DeviceID:      "dfa95244-853f-0beb-bd55-8ee656c65691",
		UserSessionID: "c6dd7c94-2aa8-3e74-c085-b45349ea7872",
		QratorJSID:    "1777298004.398.dMtyxNXb4sgzilFy-j2ueo25a5t9c2rc7rcne1ig2shit0v18",
	},
	{
		// Пермь — НОВЫЕ токены из инкогнито
		CitySlug:      "perm",
		CityName:      "Пермь",
		StoreID:       3567,
		StoreName:     "ТК297 — Героев Хасана, 105 (ТЦ Шоколад)",
		Type:          "pickup",
		SessionToken:  "45981F164978A81AE518D9E05309AB85",
		DeviceID:      "82c452a4-d201-6f33-a9ed-d35b9cc0d24d",
		UserSessionID: "b05e9f1d-0267-714b-3f91-0ce75f71d077",
		QratorJSID:    "1777307527.995.oZKaqnMDBENbpWDK-oso6s68vomhbgmvi3812o5o22kdqq9d5",
	},
}

type RequestBody struct {
	CategoryID int            `json:"categoryId"`
	Filters    map[string]any `json:"filters"`
	Sort       map[string]any `json:"sort"`
	Limit      int            `json:"limit"`
	Offset     int            `json:"offset"`
}

type APIResponse struct {
	Items []Item `json:"items"`
	Total int    `json:"total"`
}

type Item struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Prices Prices `json:"prices"`
}

type Prices struct {
	Price        int `json:"price"`
	PriceRegular int `json:"priceRegular"`
}

type ProductResult struct {
	Name     string `json:"name"`
	Price    string `json:"price"`
	URL      string `json:"url"`
	Category string `json:"category"`
}

type Parser struct {
	store  StoreConfig
	client *http.Client
}

func NewParser(proxyURL string, store StoreConfig) *Parser {
	transport := &http.Transport{}

	if proxyURL != "" {
		proxy, err := url.Parse(proxyURL)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxy)
			fmt.Printf("🔒 Прокси: %s\n", proxyURL)
		}
	}

	// Проверяем, что токены заполнены
	if store.SessionToken == "" || store.SessionToken == "НУЖНО_ЗАМЕНИТЬ" {
		fmt.Println("⚠️  ВНИМАНИЕ: Токены для этого магазина не настроены!")
		fmt.Println("   Данные могут не загрузиться.")
	}

	return &Parser{
		store: store,
		client: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}
}

func (p *Parser) setHeaders(req *http.Request) {
	req.Header.Set("accept", "application/json")
	req.Header.Set("accept-encoding", "gzip, deflate, br, zstd")
	req.Header.Set("accept-language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("client", "angular_web_0.0.2")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("deviceid", p.store.DeviceID)
	req.Header.Set("origin", "https://lenta.com")
	req.Header.Set("referer", "https://lenta.com/catalog/")
	req.Header.Set("sec-ch-ua", `"Google Chrome";v="147", "Not.A/Brand";v="8", "Chromium";v="147"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "same-origin")
	req.Header.Set("sessiontoken", p.store.SessionToken)
	req.Header.Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36")
	req.Header.Set("x-delivery-mode", p.store.Type)
	req.Header.Set("x-device-id", p.store.DeviceID)
	req.Header.Set("x-device-os", "Web")
	req.Header.Set("x-device-os-version", "12.4.8")
	req.Header.Set("x-domain", p.store.CitySlug)
	req.Header.Set("x-platform", "omniweb")
	req.Header.Set("x-retail-brand", "lo")
	req.Header.Set("x-user-session-id", p.store.UserSessionID)

	// Cookie с данными магазина
	missionAddress := fmt.Sprintf(
		`{"t":"%s","ids":true,"ma":{"i":%d,"a":"%04d","t":"%s","ri":1,"mt":"HM","s":false}}`,
		p.store.Type, p.store.StoreID, p.store.StoreID, p.store.StoreName,
	)
	missionEncoded := url.QueryEscape(missionAddress)

	req.Header.Set("cookie",
		"qrator_jsid="+p.store.QratorJSID+"; "+
			"App_Cache_CitySlug="+p.store.CitySlug+"; "+
			"UserSessionId="+p.store.UserSessionID+"; "+
			"Utk_SessionToken="+p.store.SessionToken+"; "+
			"deviceid="+p.store.DeviceID+"; "+
			"Utk_DvcGuid="+p.store.DeviceID+"; "+
			"App_Cache_MissionAddressMode="+missionEncoded)
}

func (p *Parser) FetchCategory(categoryID int, categoryName string) ([]ProductResult, error) {
	var allProducts []ProductResult

	for offset := 0; offset < 10000; offset += 40 {
		fmt.Printf("  [offset=%d]", offset)

		var products []ProductResult
		var total int
		var err error

		for retry := 0; retry < 3; retry++ {
			products, total, err = p.fetchPage(categoryID, offset)
			if err == nil {
				break
			}

			errMsg := err.Error()
			if errMsg == "rate_limited" || errMsg == "forbidden" {
				waitTime := time.Duration(retry+1) * 10 * time.Second
				fmt.Printf("\n  ⚠️ %s, ждем %v...", errMsg, waitTime)
				time.Sleep(waitTime)
				continue
			}
			return nil, err
		}

		if err != nil {
			break
		}

		for i := range products {
			products[i].Category = categoryName
		}
		allProducts = append(allProducts, products...)
		fmt.Printf(" +%d (всего: %d/%d)\n", len(products), len(allProducts), total)

		if len(products) < 40 || len(allProducts) >= total {
			break
		}
		time.Sleep(2 * time.Second)
	}
	return allProducts, nil
}

func (p *Parser) fetchPage(categoryID, offset int) ([]ProductResult, int, error) {
	body := RequestBody{
		CategoryID: categoryID,
		Filters:    map[string]any{"checkbox": []any{}, "multicheckbox": []any{}, "range": []any{}},
		Sort:       map[string]any{"type": "popular", "order": "desc"},
		Limit:      40,
		Offset:     offset,
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "https://lenta.com/api-gateway/v1/catalog/items", bytes.NewBuffer(jsonBody))
	p.setHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 429:
		return nil, 0, fmt.Errorf("rate_limited")
	case 403:
		bodyBytes, _ := io.ReadAll(resp.Body)
		fmt.Printf("\n  🔴 403 Ответ: %s\n", string(bodyBytes[:min(200, len(bodyBytes))]))
		return nil, 0, fmt.Errorf("forbidden")
	case 200:
		// OK
	default:
		bodyBytes, _ := io.ReadAll(resp.Body)
		fmt.Printf("\n  🔴 %d Ответ: %s\n", resp.StatusCode, string(bodyBytes[:min(200, len(bodyBytes))]))
		return nil, 0, fmt.Errorf("статус: %d", resp.StatusCode)
	}

	var reader io.ReadCloser
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, _ = gzip.NewReader(resp.Body)
		defer reader.Close()
	} else {
		reader = resp.Body
	}

	var apiResp APIResponse
	if err := json.NewDecoder(reader).Decode(&apiResp); err != nil {
		return nil, 0, err
	}

	var products []ProductResult
	for _, item := range apiResp.Items {
		price := float64(item.Prices.Price) / 100
		products = append(products, ProductResult{
			Name:  item.Name,
			Price: fmt.Sprintf("%.2f ₽", price),
			URL:   fmt.Sprintf("https://lenta.com/product/%s-%d/", item.Slug, item.ID),
		})
	}

	return products, apiResp.Total, nil
}

func SaveResults(prefix string, products []ProductResult) {
	jsonData, _ := json.MarshalIndent(products, "", "  ")
	os.WriteFile(prefix+".json", jsonData, 0644)

	file, _ := os.Create(prefix + ".csv")
	defer file.Close()
	file.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(file)
	writer.Comma = ';'
	writer.Write([]string{"Категория", "Наименование", "Цена", "Ссылка"})
	for _, p := range products {
		writer.Write([]string{p.Category, p.Name, p.Price, p.URL})
	}
	writer.Flush()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	fmt.Println("╔══════════════════════════════════╗")
	fmt.Println("║   Парсер товаров Lenta.com      ║")
	fmt.Println("╚══════════════════════════════════╝")
	fmt.Println()

	fmt.Println("Доступные магазины:")
	for i, s := range knownStores {
		status := "✅"
		if s.SessionToken == "" || s.SessionToken == "НУЖНО_ЗАМЕНИТЬ" {
			status = "⚠️ (нет токенов)"
		}
		fmt.Printf("  [%d] %s %s — %s (ID: %d)\n", i+1, status, s.CityName, s.StoreName, s.StoreID)
	}
	fmt.Println()

	// Выбор магазина
	selectedIdx := 0
	if len(os.Args) > 1 {
		idx, err := strconv.Atoi(os.Args[1])
		if err == nil && idx > 0 && idx <= len(knownStores) {
			selectedIdx = idx - 1
		}
	}
	selectedStore := knownStores[selectedIdx]

	if selectedStore.SessionToken == "" || selectedStore.SessionToken == "НУЖНО_ЗАМЕНИТЬ" {
		fmt.Printf("❌ Ошибка: для магазина '%s' не настроены токены!\n", selectedStore.StoreName)
		fmt.Println("   Как получить токены:")
		fmt.Println("   1. Откройте lenta.com в браузере")
		fmt.Println("   2. Выберите нужный город и магазин")
		fmt.Println("   3. F12 → Application → Cookies → lenta.com")
		fmt.Println("   4. Скопируйте: Utk_SessionToken, UserSessionId, Utk_DvcGuid, qrator_jsid")
		fmt.Println("   5. Вставьте их в массив knownStores в коде")
		return
	}

	fmt.Printf("✅ Выбран: %s — %s\n\n", selectedStore.CityName, selectedStore.StoreName)

	// Прокси
	proxyURL := "" // "http://user:pass@proxy:8080"
	parser := NewParser(proxyURL, selectedStore)

	fmt.Printf("📍 Город: %s\n", parser.store.CityName)
	fmt.Printf("🏪 Магазин: %s (ID: %d)\n", parser.store.StoreName, parser.store.StoreID)
	fmt.Printf("🚚 Способ: %s\n", parser.store.Type)
	fmt.Printf("🔑 Токен: %s...\n\n", parser.store.SessionToken[:20])

	// Категории
	categories := map[string]int{
		"Готовая еда":    42,
		"Сыры":           2,
		"Овощи и фрукты": 144,
	}

	for name, catID := range categories {
		fmt.Printf("=== Категория: %s (ID: %d) ===\n", name, catID)
		products, err := parser.FetchCategory(catID, name)
		if err != nil {
			fmt.Printf("❌ Ошибка: %v\n", err)
			continue
		}
		if len(products) > 0 {
			SafeFileName := strings.ReplaceAll(name, " ", "_")
			SaveResults(SafeFileName, products)
			fmt.Printf("✅ Сохранено %d товаров в %s.{json,csv}\n\n", len(products), SafeFileName)
		}
		time.Sleep(10 * time.Second)
	}

	fmt.Println("=== Готово! ===")
}
