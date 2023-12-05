package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

const TABLE_SIZE int = 512

type Element struct {
	key   string
	value string
}

type HashMap struct {
	hashmap [TABLE_SIZE]*Element
	mutex   sync.Mutex
}

func HashFunc(key string) int {
	sum := 0
	for _, char := range key {
		sum += int(char)
	}
	return sum % TABLE_SIZE
}

type Stats struct {
	ID          int
	Pid         int
	RedirectURL string    `json:"redirect_url"`
	IPAddress   string    `json:"ip_address"`
	Timestamp   time.Time `json:"timestamp"`
	Count       int
}

type StatsMap struct {
	hashmap [TABLE_SIZE]*Stats
	mutex   sync.Mutex
}

// вставка нового элемента в хеш-таблицу
func (hmap *HashMap) insert(key string, value string) error {
	Element := &Element{key: key, value: value}
	index := HashFunc(key)
	if hmap.hashmap[index] == nil {
		hmap.hashmap[index] = Element
		return nil
	} else {
		if hmap.hashmap[index].key == key {
			hmap.hashmap[index] = Element
			return nil
		}
		index++
		for i := 0; i < TABLE_SIZE; i++ {
			if index == TABLE_SIZE {
				index = 0
			}
			if hmap.hashmap[index] == nil {
				hmap.hashmap[index] = Element
				return nil
			}
			index++
		}
	}
	fmt.Println("Недостаточно пространства") //выводится сообщение об ошибке и возвращается соответствующая ошибка
	return errors.New("недостаточно пространства")
}

func (smap *StatsMap) insert(key string, stats *Stats) error {
	index := HashFunc(key)
	if smap.hashmap[index] == nil {
		smap.hashmap[index] = stats
		return nil
	} else {
		index++
		for i := 0; i < TABLE_SIZE; i++ {
			if index == TABLE_SIZE {
				index = 0
			}
			if smap.hashmap[index] == nil {
				smap.hashmap[index] = stats
				return nil
			}
			index++
		}
	}
	return errors.New("недостаточно пространства")
}
func (hmap *HashMap) get(key string) (string, error) {
	index := HashFunc(key)

	if hmap.hashmap[index] == nil || hmap.hashmap[index].key != key {
		return "", errors.New("элемент не найден")
	}

	return hmap.hashmap[index].value, nil
}

func (smap *StatsMap) get(key string) (*Stats, error) {
	index := HashFunc(key)

	if smap.hashmap[index] == nil {
		return nil, errors.New("элемент не найден")
	}

	return smap.hashmap[index], nil
}

func parser(conn net.Conn, command string, hashmap *HashMap, linkShort *HashMap, statsMap *StatsMap, ipAddress string) {
	commandParts := strings.Fields(command)

	if len(commandParts) < 2 {
		fmt.Fprintln(conn, "Недостаточно аргументов")
		return
	}

	key := commandParts[1]
	switch commandParts[0] {
	case "HSET":
		if len(commandParts) != 3 {
			fmt.Fprintln(conn, "Недостаточно аргументов")
			return
		}
		value := commandParts[2]

		hashmap.mutex.Lock()
		linkShort.mutex.Lock()
		defer hashmap.mutex.Unlock()
		defer linkShort.mutex.Unlock()

		short, err := linkShort.get(value)

		if err == nil {
			log.Println("Такая ссылка существует")
			conn.Write([]byte("This link is already in our base: " + short + "\n"))
			return
		}

		if err := hashmap.insert(key, value); err != nil {
			fmt.Fprintln(conn, "Ошибка вставки:", err)
			return
		}

		linkShort.insert(value, key)

		fmt.Fprintln(conn, value)
	case "HGET":
		hashmap.mutex.Lock()
		defer hashmap.mutex.Unlock()

		if value, err := hashmap.get(key); err != nil {
			log.Println("Ошибка получения значения:", err.Error())
			conn.Write([]byte("No link found!\n"))
			return
		} else {
			fmt.Fprintln(conn, value)
			stats := &Stats{
				RedirectURL: value,
				IPAddress:   ipAddress,
				Timestamp:   time.Now(),
			}
			statsMap.insert(value, stats)
			log.Printf("Статистика добавлена: %s\n", value)
			log.Printf("IP: %s\n", ipAddress)
			log.Printf("Time: %s\n", time.Now())

			return
		}
	case "HSTATS":
		statsMap.mutex.Lock()
		defer statsMap.mutex.Unlock()

		if stats, err := statsMap.get(key); err != nil {
			log.Println("Ошибка получения статистики:", err.Error())
			conn.Write([]byte("No stats found!\n"))
			return
		} else {
			statsJSON, _ := json.Marshal(stats)
			fmt.Fprintln(conn, string(statsJSON))
			return
		}
	default:
		fmt.Fprintln(conn, "Недопустимая команда")
		return
	}
}
func readStatisticsFromFile(filename string) ([]Stats, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var statistics []Stats

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		var stats Stats
		if err := json.Unmarshal([]byte(line), &stats); err != nil {
			return nil, err
		}

		statistics = append(statistics, stats)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return statistics, nil
}

func main() {
	hashMap := &HashMap{}
	linkShort := &HashMap{}
	statsMap := &StatsMap{}

	listener, err := net.Listen("tcp", ":6328") // метод net.Listen используется для создания слушателя, который принимает входящие соединения

	if err != nil {
		fmt.Println("Ошибка при запуске сервера:", err)
		return
	}

	// Чтение статистики из файла и добавление в statsMap
	statistics, err := readStatisticsFromFile("statistics.json")
	if err != nil {
		fmt.Println("Ошибка чтения статистики из файла:", err)
		return
	}
	for _, stat := range statistics {
		statsMap.insert(stat.RedirectURL, &stat)
	}

	defer listener.Close()
	fmt.Println("Сервер запущен.")
	for {
		conn, err := listener.Accept() //сервер ожидает подключения клиента
		if err != nil {
			fmt.Println("Ошибка при принятии соединения:", err)
			conn.Close()
			continue
		}
		ipAddress := conn.RemoteAddr().String() // получаем IP-адрес подключившегося клиента
		go handleConnection(conn, hashMap, linkShort, statsMap, ipAddress)
	}
}

func handleConnection(conn net.Conn, hashMap *HashMap, linkShort *HashMap, statsMap *StatsMap, ipAddress string) { //функция handleConnection отвечает за обработку входящего соединения
	defer conn.Close() //соединение будет закрыто после завершения функции
	source := bufio.NewScanner(conn)

	for source.Scan() {
		command := source.Text()
		parser(conn, command, hashMap, linkShort, statsMap, ipAddress)
	}
}
