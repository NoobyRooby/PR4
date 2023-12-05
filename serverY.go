package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	mutex      sync.Mutex
	statistics []Stat
)

type Stat struct {
	ID           int    `json:"id"`
	URL          string `json:"url"`
	ShortUrl     string `json:"shorturl"`
	SourceIP     string `json:"ipsrc"`
	TimeInterval string `json:"time"`
}

func shortenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "error", http.StatusMethodNotAllowed)
		return
	}
	url, err := io.ReadAll(r.Body)

	conn, err := net.Dial("tcp", "localhost:6328")
	if err != nil {
		fmt.Println("Не удалось подключиться к серверу:", err)
		os.Exit(1)
	}
	defer conn.Close()

	mutex.Lock()
	defer mutex.Unlock()

	shortURL := generateShortURL()

	fmt.Printf("URL: [%s]\t Short: [%s]\n", string(url), shortURL)

	_, err = conn.Write([]byte("HSET " + shortURL + " " + string(url) + "\n"))
	if err != nil {
		fmt.Println("Не удалось отправить команду на сервер:", err)
		return
	}

	resp, err := bufio.NewReader(conn).ReadString('\n')

	log.Printf("resp: [%s]\n", resp)

	if strings.Contains(resp, "This link is already in our base") {
		log.Println("This link is already in our base!")
		fmt.Fprintf(w, resp)
		return
	}

	log.Println("Passed!")
	fmt.Fprintf(w, "Сокращенная ссылка: http://localhost:8010/%s", shortURL)
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := net.Dial("tcp", "localhost:6328")
	if err != nil {
		fmt.Println("Не удалось подключиться к серверу:", err)
		os.Exit(1)
	}
	defer conn.Close()

	mutex.Lock()
	defer mutex.Unlock()

	shortURL := strings.TrimPrefix(r.URL.Path, "/")

	if shortURL == "" || shortURL == "favicon.ico" {
		conn.Write([]byte("nothing"))
		return
	}

	fmt.Printf("URL: [%s]\t Short: [%s]\n", string(r.URL.Path), shortURL)

	_, err = conn.Write([]byte("HGET " + shortURL + "\n"))
	if err != nil {
		fmt.Println("Не удалось отправить команду на сервер:", err)
		http.NotFound(w, r)
		return
	}

	log.Println("Died here!")

	resp, err := bufio.NewReader(conn).ReadString('\n')

	log.Println(resp)

	if err != nil {
		fmt.Fprintf(w, "Server Error!")
		return
	}

	log.Printf("resp: [%s]\n", resp)

	if resp == "Эта ссылка уже есть!\n" {
		log.Println("This link is already in our base!")
		fmt.Fprintf(w, "This link is already in our base!")
		return
	}

	originalURL := resp

	http.Redirect(w, r, "/", http.StatusFound)
	exec.Command("cmd.exe", "/C", "start", originalURL).Run()

	timest := time.Now().Format(time.RFC3339)

	sendStatisticToAggregationServer(originalURL, shortURL, r.RemoteAddr, timest)
}

func generateShortURL() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 7)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func sendStatisticToAggregationServer(url, shorturl, ip string, timest string) {
	log.Printf("Sending statistic: url=%s, shorturl=%s, ip=%s, time=%s\n", url, shorturl, ip, timest)
	statistic := Stat{
		ID:           len(statistics) + 1,
		URL:          url,
		ShortUrl:     shorturl,
		SourceIP:     ip,
		TimeInterval: timest,
	}

	data, err := json.Marshal(statistic)
	if err != nil {
		log.Println("Не удалось преобразовать статистику в JSON:", err)
		return
	}

	file, err := os.OpenFile("statistics.json", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println("Не удалось открыть файл для записи статистики:", err)
		return
	}
	defer file.Close()

	if _, err := file.Write(append(data, []byte("\n")...)); err != nil {
		log.Println("Не удалось записать статистику в файл:", err)
		return
	}

	log.Println("Статистика успешно записана в файл")

	client := &http.Client{}
	reqBody := strings.NewReader(string(data))
	req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/", reqBody)
	if err != nil {
		log.Println("Не удалось создать HTTP-запрос:", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Не удалось выполнить HTTP-запрос:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Ошибка при отправке статистики на сервер. Код состояния: %d\n", resp.StatusCode)
		return
	}

	log.Println("Статистика успешно отправлена на сервер агрегации")
}

func main() {
	http.HandleFunc("/shorten", shortenHandler)
	http.HandleFunc("/", redirectHandler)
	fmt.Println("Сервер запущен на порту 8010")
	err := http.ListenAndServe(":8010", nil)
	if err != nil {
		fmt.Println("Ошибка запуска сервера:", err)
	}
}
