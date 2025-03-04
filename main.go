// @title Songs Api
// @version 1.0
// @description тестовое задание на позицию Go-разработчика

// @contact.email kseni.golosovskaya@gmail.com

// @host localhost:8080
// @BasePath /api/v1
// main.go
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	_ "mySample-go-app/docs"
	//"github.com/swaggo/gin-swagger"
)

// Song – модель песни в базе данных
type Song struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Group       string `json:"group"`
	Song        string `json:"song"`
	ReleaseDate string `json:"releaseDate"`
	Text        string `json:"text"`
	Link        string `json:"link"`
}

// APIResponse – структура ответа внешнего API (согласно сваггеру)
type APIResponse struct {
	ReleaseDate string `json:"releaseDate"`
	Text        string `json:"text"`
	Link        string `json:"link"`
}

// InputSong – структура входящих данных для создания/обновления песни
type InputSong struct {
	Group string `json:"group" binding:"required"`
	Song  string `json:"song" binding:"required"`
}

// loadEnv загружает конфигурацию из файла .env и выводит  лог с загруженными ключами
func loadEnv() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("[ERROR] При загрузке .env файла произошла ошибка: %v", err)
	}
	log.Println("[INFO] Конфигурация загружена из .env")
	// В лог выводим все переменные окружения, которые могут быть полезны
	keys := []string{"DATABASE_URL", "EXTERNAL_API_URL", "PORT"}
	for _, key := range keys {
		val := os.Getenv(key)
		log.Printf("[INFO] %s=%s", key, val)
	}
}

// loadtToEnv выносит текущие конфигурационные параметры в файл .env
func loadtToEnv() {
	// Укажите ключи, которые хотите сохранить
	keys := []string{"DATABASE_URL", "EXTERNAL_API_URL", "PORT"}
	file, err := os.Create(".env")
	if err != nil {
		log.Printf("[ERROR] Произошла ошибка попытке создания .env файла: %v", err)
		return
	}
	defer file.Close()

	for _, key := range keys {
		val := os.Getenv(key)
		line := fmt.Sprintf("%s=%s\n", key, val)
		if _, err := file.WriteString(line); err != nil {
			log.Printf("[ERROR] Ошибка при записи для ключа %s: %v", key, err)
		} else {
			log.Printf("[INFO] Записана переменная %s", key)
		}
	}
	log.Println("[INFO] Конфигурационные параметры выгружены в .env файл")
}

// initDB устанавливает соединение с Postgres, логгирует DEBUG информацию и в реализует миграцию
func initDB() *gorm.DB {
	dsn := os.Getenv("DATABASE_URL")
	log.Printf("[DEBUG] Используем DSN: %s", dsn)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("[ERROR] Ошибка при попытке подключения к базе данных: %v", err)
	}
	if err = db.AutoMigrate(&Song{}); err != nil {
		log.Fatalf("[ERROR] Ошибка миграции базы данных: %v", err)
	}
	log.Println("[INFO] Соединение с бд успешно установлено. Миграции выполнены.")
	return db
}

// extApiRequest отправляет запрос к внешнему API для обогащения данных песни
func extApiRequest(songGroup, songName string) (*APIResponse, error) {
	baseURL := os.Getenv("EXTERNAL_API_URL")
	// Формируем URL согласно сваггеру: /info?group=...&song=...
	url := fmt.Sprintf("%s/info?group=%s&song=%s", baseURL, songGroup, songName)
	log.Printf("[DEBUG] Запрос к внешнему API: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("[ERROR] Ошибка запроса к внешнему API: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		log.Printf("[ERROR] Внешний API вернул ошибку: %s", string(bodyBytes))
		return nil, fmt.Errorf("статус ответа от внешнего API: %d", resp.StatusCode)
	}

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		log.Printf("[ERROR] Ошибка декодирования ответа внешнего API: %v", err)
		return nil, err
	}
	log.Printf("[DEBUG] Получены данные от внешнего API: %+v", apiResp)
	log.Printf("[INFO] Получены обогащенные данные для группы '%s' и песни '%s'", songGroup, songName)
	return &apiResp, nil
}

// paginateText выполняет паггинацию  песни по куплетам с разделителем "\n\n" и возвращает часть для запрошенной страницы
func paginateText(text string, page, pageSize int) string {
	couplts := strings.Split(text, "\n\n")
	start := (page - 1) * pageSize
	if start >= len(couplts) {
		log.Printf("[DEBUG] Начальный индекс %d больше или равен количеству куплетов %d", start, len(couplts))
		return ""
	}
	end := start + pageSize
	if end > len(couplts) {
		end = len(couplts)
	}
	log.Printf("[DEBUG] Пагинация текста: страница %d, размер страницы %d, куплетов с %d по %d", page, pageSize, start, end)
	return strings.Join(couplts[start:end], "\n\n")
}

func main() {
	// Вызов функции для загрузки конфигураций из .env файла
	loadEnv()

	// Вынзов функции для выноса текущих конфигураций в .env файл
	loadtToEnv()

	// Инициализируем базу данных
	db := initDB()

	router := gin.Default()

	// Получение файловой библиотеки с фильтрацией по полям и пагинацией
	// Фильтрация по: group, song, releaseDate; пагинация по: page и pageSize
	router.GET("/library", func(c *gin.Context) {
		var songs []Song
		query := db

		if groups := c.Query("group"); groups != "" {
			log.Printf("[DEBUG] Фильтрация по группе: %s", groups)
			query = query.Where("group ILIKE ?", "%"+groups+"%")
		}
		if songName := c.Query("song"); songName != "" {
			log.Printf("[DEBUG] Фильтрация по названию песни: %s", songName)
			query = query.Where("song ILIKE ?", "%"+songName+"%")
		}
		if releaseDate := c.Query("releaseDate"); releaseDate != "" {
			log.Printf("[DEBUG] Фильтрация по дате релиза: %s", releaseDate)
			query = query.Where("release_date = ?", releaseDate)
		}

		//Паггинация
		page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
		if err != nil || page < 1 {
			log.Printf("[DEBUG] Некорректный параметр page, устанавливаем в 1")
			page = 1
		}
		pageSize, err := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
		if err != nil || pageSize < 1 {
			log.Printf("[DEBUG] Некорректный параметр pageSize, устанавливаем в 10")
			page = 10
		}
		offset := (page - 1) * pageSize
		log.Printf("[DEBUG] Пагинация: страница %d, размер %d, offset %d", page, pageSize, offset)

		// Выполнение запроса с учетом фильтров и пагинации
		if err := query.Limit(pageSize).Offset(offset).Find(&songs).Error; err != nil {
			log.Printf("[ERROR] Ошибка получения данных библиотеки: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения данных"})
			return
		}

		c.JSON(http.StatusOK, songs)
	})

	// rest-метод для добавления новой песни в базу данных
	router.POST("/library", func(c *gin.Context) {
		var newSong Song
		// Привязка JSON данных к структуре Song
		if err := c.ShouldBindJSON(&newSong); err != nil {
			log.Printf("[DEBUG] Ошибка при парсинге данных: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
			return
		}

		// Добавление новой песни в базу
		if err := db.Create(&newSong).Error; err != nil {
			log.Printf("[ERROR] Ошибка добавления песни в базу: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось добавить песню"})
			return
		}

		c.JSON(http.StatusCreated, newSong)
	})
	// rest-метод для обновления информации  песни
	router.PUT("/library/:id", func(c *gin.Context) {
		// Получение идентификатора песни из параметров URL
		id := c.Param("id")
		var song Song

		// Поиск песни в базе данных
		if err := db.First(&song, id).Error; err != nil {
			log.Printf("[ERROR] Песня с ID %s не найдена: %v", id, err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Песня не найдена"})
			return
		}

		// Привязка JSON данных с обновлениями к новой структуре
		var updatedData Song
		if err := c.ShouldBindJSON(&updatedData); err != nil {
			log.Printf("[DEBUG] Ошибка при парсинге обновляемых данных: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
			return
		}

		// Обновление данных песни
		if err := db.Model(&song).Updates(updatedData).Error; err != nil {
			log.Printf("[ERROR] Ошибка обновления песни с ID %s: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось обновить песню"})
			return
		}

		c.JSON(http.StatusOK, song)
	})

	// rest-метод для удаления песни
	router.DELETE("/library/:id", func(c *gin.Context) {
		// Получение идентификатора песни из параметров URL
		id := c.Param("id")
		var song Song

		// Поиск песни в базе данных
		if err := db.First(&song, id).Error; err != nil {
			log.Printf("[ERROR] Песня с ID %s не найдена: %v", id, err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Песня не найдена"})
			return
		}

		// Удаление песни из базы данных
		if err := db.Delete(&song).Error; err != nil {
			log.Printf("[ERROR] Ошибка удаления песни с ID %s: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось удалить песню"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Песня успешно удалена"})
	})

}
