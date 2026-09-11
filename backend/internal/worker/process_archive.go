package worker

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/airsss993/histproject-backend/internal/config"
	"github.com/airsss993/histproject-backend/internal/requests"
	"github.com/airsss993/histproject-backend/pkg/storage"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/dutchcoders/go-clamd"
	"github.com/hibiken/asynq"
)

// Worker — обработка фоновых задач (архивы заявок).
type Worker struct {
	requestRepo requests.RequestRepository
	storage     *storage.MinioClient
	cfg         config.Config
	chromeMu    sync.Mutex
}

// NewWorker создаёт Worker с репозиторием заявок и хранилищем.
func NewWorker(requestRepo requests.RequestRepository, storage *storage.MinioClient, cfg config.Config) *Worker {
	return &Worker{
		requestRepo: requestRepo,
		storage:     storage,
		cfg:         cfg,
	}
}

// NewMux - функция для создания нового ServeMux и регистрации обработчиков задач
func NewMux(w *Worker) *asynq.ServeMux {
	mux := asynq.NewServeMux()

	mux.HandleFunc(TypeProcessArchive, w.ProcessArchiveTask)

	return mux
}

// ProcessArchiveTask - обработчик задачи по обработке архива
func (w *Worker) ProcessArchiveTask(ctx context.Context, t *asynq.Task) error {
	// Получение ID заявки и архива
	var payload ProcessArchivePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("ошибка при разборе полезной нагрузки задачи: %w", err)
	}

	log.Printf("[worker] requestId=%d archiveId=%s: начало обработки", payload.RequestId, payload.ArchiveId)

	// Получаем slug заявки для формирования URL и пути в хранилище
	req, err := w.requestRepo.GetByID(ctx, payload.RequestId)
	if err != nil {
		return fmt.Errorf("ошибка получения заявки: %w", err)
	}
	siteSlug := req.Slug

	// Получение архива из хранилища по ID архива
	archive, _, err := w.storage.GetArchive(payload.ArchiveId)
	if err != nil {
		return fmt.Errorf("ошибка при получении архива из хранилища: %w", err)
	}
	defer archive.Close()

	if w.cfg.App.ClamAVEnabled {
		log.Printf("[worker] requestId=%d: проверка на вирусы", payload.RequestId)
		if err := w.checkArchiveForViruses(archive); err != nil {
			log.Printf("[worker] requestId=%d: обнаружен вирус: %v", payload.RequestId, err)
			_ = w.requestRepo.UpdateStatus(ctx, payload.RequestId, "Отклонена", "Обнаружен вирус", "", "")
			return nil
		}
	}

	log.Printf("[worker] requestId=%d: распаковка архива", payload.RequestId)
	if err := w.unzipArchive(siteSlug, payload.ArchiveId); err != nil {
		log.Printf("[worker] requestId=%d: ошибка распаковки: %v", payload.RequestId, err)
		_ = w.requestRepo.UpdateStatus(ctx, payload.RequestId, "Отклонена", "Отсутствует index.html", "", "")
		return nil
	}

	log.Printf("[worker] requestId=%d: создание скриншота", payload.RequestId)
	w.chromeMu.Lock()
	screenshotUrl, err := w.createScreenshot(siteSlug)
	w.chromeMu.Unlock()
	if err != nil {
		log.Printf("[worker] requestId=%d: ошибка скриншота: %v", payload.RequestId, err)
		return fmt.Errorf("ошибка при создании скриншота: %w", err)
	}

	siteUrl := fmt.Sprintf("http://%s/preview/%s/index.html", w.cfg.Storage.SitePublicUrl, siteSlug)
	log.Printf("[worker] requestId=%d: готово, siteUrl=%s screenshotUrl=%s", payload.RequestId, siteUrl, screenshotUrl)

	// Используем свежий контекст — asynq-контекст к этому моменту может быть отменён
	saveCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := w.requestRepo.UpdateStatus(saveCtx, payload.RequestId, "Новая", "", siteUrl, screenshotUrl); err != nil {
		return fmt.Errorf("ошибка сохранения результата в БД: %w", err)
	}

	return nil
}

func (w *Worker) createScreenshot(siteSlug string) (string, error) {
	chromeAddr := strings.Replace(w.cfg.App.ChromeUrl, "ws://", "", 1)
	httpClient := &http.Client{Timeout: 15 * time.Second}

	// Закрываем все зависшие вкладки Chrome от предыдущих упавших сессий.
	// Без этого накопленные вкладки перегружают Chrome и /json/version начинает отвечать за ~50с.
	w.closeChromeTargets(httpClient, chromeAddr)

	req, err := http.NewRequest("GET", "http://"+chromeAddr+"/json/version", nil)
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса к Chrome: %w", err)
	}
	req.Host = "localhost"

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка подключения к Chrome: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Разбираем JSON-ответ от Chrome для получения webSocketDebuggerUrl
	var result map[string]string
	json.Unmarshal(body, &result)

	// Запускаем локальный TCP-прокси 127.0.0.1:RANDOM → chromeAddr.
	// Chrome отвергает WebSocket-соединения с hostname в Host-заголовке (например "chrome:9222"),
	// но принимает IP-адреса. Прокси позволяет использовать "127.0.0.1:RANDOM" в URL.
	proxyAddr, stopProxy, err := startLocalProxy(chromeAddr)
	if err != nil {
		return "", fmt.Errorf("ошибка запуска прокси к Chrome: %w", err)
	}
	defer stopProxy()
	log.Printf("[worker] createScreenshot: proxy %s → %s", proxyAddr, chromeAddr)

	parsedWsURL, _ := url.Parse(result["webSocketDebuggerUrl"])
	parsedWsURL.Host = proxyAddr
	wsURL := parsedWsURL.String()
	log.Printf("[worker] createScreenshot: wsURL=%s", wsURL)

	// Создание контекста для chromedp
	allocatorContext, cancel := chromedp.NewRemoteAllocator(
		context.Background(),
		wsURL,
		chromedp.NoModifyURL,
	)
	defer cancel()

	// Создание контекста для выполнения задач chromedp с таймаутом 60 секунд
	ctx, cancel := chromedp.NewContext(allocatorContext)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var buf []byte
	siteUrl := fmt.Sprintf("http://%s/sites/%s/index.html", w.cfg.Storage.MinioEndpoint, siteSlug)

	// Выставляем размер скриншота 1920x1080 и делаем скриншот только видимой области.
	if err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, _, _, _, err := page.Navigate(siteUrl).Do(ctx)
			return err
		}),
		chromedp.Sleep(7*time.Second),
		chromedp.ActionFunc(func(ctx context.Context) error {
			data, err := page.CaptureScreenshot().WithQuality(90).Do(ctx)
			if err != nil {
				return err
			}
			buf = data
			return nil
		}),
	); err != nil {
		return "", fmt.Errorf("ошибка при создании скриншота: %w", err)
	}

	// Загрузка скриншота в хранилище и получение URL
	screenshotKey := fmt.Sprintf("%s/screenshot.png", siteSlug)
	if err := w.storage.UploadFileSites(bytes.NewReader(buf), int64(len(buf)), screenshotKey); err != nil {
		return "", fmt.Errorf("ошибка при загрузке скриншота в хранилище: %w", err)
	}

	// Формирование URL скриншота
	screenshotUrl := fmt.Sprintf("http://%s/sites/%s/screenshot.png", w.cfg.Storage.MinioPublicUrl, siteSlug)

	return screenshotUrl, nil
}

// unzipArchive - функция для распаковки архива и загрузки файлов в хранилище
func (w *Worker) unzipArchive(siteSlug string, archiveID string) error {
	// Получить объект из MinIO и его размер
	archive, size, err := w.storage.GetArchive(archiveID)
	if err != nil {
		return fmt.Errorf("ошибка при получении архива из хранилища: %w", err)
	}
	defer archive.Close()

	// Открыть как zip через zip.NewReader(object, size)
	zipReader, err := zip.NewReader(archive, size)
	if err != nil {
		return fmt.Errorf("ошибка при открытии архива как zip: %w", err)
	}

	// Проверить наличие index.html в корне или на первом уровне вложенности
	hasIndex := false
	var prefix string
	for _, file := range zipReader.File {
		name := filepath.Base(file.Name)
		parts := strings.Split(strings.Trim(file.Name, "/"), "/")
		if name == "index.html" && len(parts) <= 2 {
			hasIndex = true
			prefix = filepath.Dir(file.Name)
			if prefix == "." {
				prefix = ""
			}
			break
		}
	}
	if !hasIndex {
		return fmt.Errorf("ошибка при проверке наличия index.html в архиве: файл index.html не найден в корне или на первом уровне вложенности")
	}

	// Распаковать все файлы и загрузить их в хранилище, сохраняя структуру папок
	for _, file := range zipReader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("ошибка открытия файла: %v", err)
		}

		filePath := file.Name
		if prefix != "" {
			filePath = strings.TrimPrefix(file.Name, prefix+"/")
		}
		key := fmt.Sprintf("%s/%s", siteSlug, filePath)

		if err := w.storage.UploadFileSites(rc, int64(file.UncompressedSize64), key); err != nil {
			_ = rc.Close()
			return fmt.Errorf("ошибка загрузки файла: %v", err)
		}
		_ = rc.Close()
	}

	return nil
}

// closeChromeTargets закрывает все открытые вкладки Chrome через REST API.
// Предотвращает накопление зависших вкладок от упавших chromedp-сессий, которые перегружают браузер.
func (w *Worker) closeChromeTargets(client *http.Client, chromeAddr string) {
	req, err := http.NewRequest("GET", "http://"+chromeAddr+"/json/list", nil)
	if err != nil {
		return
	}
	req.Host = "localhost"
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var targets []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return
	}

	for _, target := range targets {
		id, ok := target["id"].(string)
		if !ok {
			continue
		}
		closeReq, err := http.NewRequest("GET", "http://"+chromeAddr+"/json/close/"+id, nil)
		if err != nil {
			continue
		}
		closeReq.Host = "localhost"
		closeResp, err := client.Do(closeReq)
		if err != nil {
			continue
		}
		closeResp.Body.Close()
	}
}

// startLocalProxy запускает TCP-прокси на случайном порту 127.0.0.1 и пробрасывает соединения на targetAddr.
func startLocalProxy(targetAddr string) (localAddr string, stop func(), err error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go proxyConn(conn, targetAddr)
		}
	}()
	return ln.Addr().String(), func() { ln.Close() }, nil
}

// proxyConn пробрасывает трафик между src и targetAddr.
func proxyConn(src net.Conn, targetAddr string) {
	defer src.Close()
	dst, err := net.Dial("tcp", targetAddr)
	if err != nil {
		return
	}
	defer dst.Close()
	done := make(chan struct{}, 2)
	go func() { io.Copy(dst, src); done <- struct{}{} }()
	go func() { io.Copy(src, dst); done <- struct{}{} }()
	<-done
}

// checkArchiveForViruses - функция для проверки архива на вирусы с помощью ClamAV
func (w *Worker) checkArchiveForViruses(reader io.Reader) error {
	// Инициализируем клиента ClamAV
	c := clamd.NewClamd(w.cfg.App.ClamAVHost)
	response, err := c.ScanStream(reader, nil)
	if err != nil {
		return fmt.Errorf("ошибка при сканировании архива на вирусы: %w", err)
	}

	// Проверяем результаты сканирования
	for result := range response {
		if result.Status == clamd.RES_FOUND {
			return fmt.Errorf("вирус найден: %s", result.Description)
		}
	}

	return nil
}
