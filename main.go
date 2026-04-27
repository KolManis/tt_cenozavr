package main

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

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
	Price int `json:"price"`
}

type ProductResult struct {
	Name     string `json:"name"`
	Price    string `json:"price"`
	URL      string `json:"url"`
	Category string `json:"category"`
}

func main() {
	categories := map[string]int{
		"Готовая еда":    42,
		"Сыры":           2,
		"Овощи и фрукты": 144,
	}

	for name, catID := range categories {
		fmt.Printf("\n=== Парсим категорию: %s (ID: %d) ===\n", name, catID)
		products := fetchCategory(catID, name)

		if len(products) > 0 {
			saveToJSON(fmt.Sprintf("%s.json", name), products)
			saveToCSV(fmt.Sprintf("%s.csv", name), products)
			fmt.Printf("✅ Сохранено %d товаров\n", len(products))
		} else {
			fmt.Printf("❌ Нет товаров\n")
		}

		time.Sleep(1 * time.Second)
	}
}

func fetchCategory(categoryID int, categoryName string) []ProductResult {
	var allProducts []ProductResult

	for offset := 0; offset < 10000; offset += 40 {
		fmt.Printf("  Загрузка offset=%d...\n", offset)

		products, total, err := fetchPage(categoryID, offset)
		if err != nil {
			fmt.Printf("  Ошибка: %v\n", err)
			break
		}

		for i := range products {
			products[i].Category = categoryName
		}

		allProducts = append(allProducts, products...)
		fmt.Printf("  Получено %d товаров (всего: %d/%d)\n", len(products), len(allProducts), total)

		if len(products) < 40 || len(allProducts) >= total {
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	return allProducts
}

func fetchPage(categoryID, offset int) ([]ProductResult, int, error) {
	body := RequestBody{
		CategoryID: categoryID,
		Filters: map[string]any{
			"checkbox":      []any{},
			"multicheckbox": []any{},
			"range":         []any{},
		},
		Sort: map[string]any{
			"type":  "popular",
			"order": "desc",
		},
		Limit:  40,
		Offset: offset,
	}

	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", "https://lenta.com/api-gateway/v1/catalog/items", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("accept-encoding", "gzip, deflate, br, zstd")
	req.Header.Set("accept-language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("client", "angular_web_0.0.2")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("deviceid", "dfa95244-853f-0beb-bd55-8ee656c65691")
	req.Header.Set("origin", "https://lenta.com")
	req.Header.Set("referer", "https://lenta.com/catalog/")
	req.Header.Set("sec-ch-ua", `"Google Chrome";v="147", "Not.A/Brand";v="8", "Chromium";v="147"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "same-origin")
	req.Header.Set("sessiontoken", "941104B44D8BF3FFBC2B354576C82F66")
	req.Header.Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("x-delivery-mode", "pickup")
	req.Header.Set("x-device-id", "dfa95244-853f-0beb-bd55-8ee656c65691")
	req.Header.Set("x-device-os", "Web")
	req.Header.Set("x-device-os-version", "12.4.8")
	req.Header.Set("x-domain", "moscow")
	req.Header.Set("x-platform", "omniweb")
	req.Header.Set("x-retail-brand", "lo")
	req.Header.Set("x-user-session-id", "c6dd7c94-2aa8-3e74-c085-b45349ea7872")
	req.Header.Set("cookie",
		"qrator_jsid=1777298004.398.dMtyxNXb4sgzilFy-j2ueo25a5t9c2rc7rcne1ig2shit0v18; "+
			"App_Cache_CitySlug=moscow; "+
			"UserSessionId=c6dd7c94-2aa8-3e74-c085-b45349ea7872; "+
			"Utk_SessionToken=941104B44D8BF3FFBC2B354576C82F66; "+
			"deviceid=dfa95244-853f-0beb-bd55-8ee656c65691; "+
			"App_Cache_MissionAddressMode=%7B%22t%22%3A%22pickup%22%2C%22ids%22%3Atrue%2C%22ma%22%3A%7B%22i%22%3A3149%2C%22a%22%3A%220124%22%7D%7D")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, 0, fmt.Errorf("статус: %d", resp.StatusCode)
	}

	var reader io.ReadCloser
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return nil, 0, err
		}
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
		product := ProductResult{
			Name:  item.Name,
			Price: fmt.Sprintf("%.2f ₽", price),
			URL:   fmt.Sprintf("https://lenta.com/product/%d-%s/", item.ID, item.Slug),
		}
		products = append(products, product)
	}

	return products, apiResp.Total, nil
}

func saveToJSON(filename string, products []ProductResult) error {
	data, err := json.MarshalIndent(products, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

func saveToCSV(filename string, products []ProductResult) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	file.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(file)
	writer.Comma = ';'
	writer.Write([]string{"Категория", "Наименование", "Цена", "Ссылка"})

	for _, p := range products {
		writer.Write([]string{p.Category, p.Name, p.Price, p.URL})
	}
	writer.Flush()
	return writer.Error()
}
