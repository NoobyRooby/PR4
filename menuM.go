package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type Dimensions struct {
	Dimensions []string `json:"dimensions"`
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)

	for {

		fmt.Println("1. Сокращение ссылки")
		fmt.Println("2. Переход по сокращенной ссылке")
		fmt.Println("3. Выход")
		fmt.Print("Введите номер команды: ")

		scanner.Scan()
		input := scanner.Text()
		switch input {
		case "1":
			{
				fmt.Println("Введите ссылку: ")
				link, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				link = link[:len(link)-2] //Удаляются символы переноса строки

				fmt.Printf("Link: [%s]\n", link) //Выводится введенная ссылка в квадратных скобках

				req, err := http.NewRequest("POST", "http://localhost:8010/shorten", bytes.NewBuffer([]byte(link))) // создается POST-запрос на сервер
				if err != nil {
					fmt.Println(err.Error())
					continue
				}

				res, err := http.DefaultClient.Do(req) //Выполняется запрос на сервер

				if err != nil {
					fmt.Println("Ошибка при отправке запроса")
					continue
				}
				defer res.Body.Close()

				body, _ := io.ReadAll(res.Body) //Читается тело ответа сервера

				fmt.Println(string(body)) //Выводится сокращенная ссылка
				break
			}
		case "2":
			fmt.Println("Введите созданное сокращение: ")
			link, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			link = strings.Trim(link, "\n")
			link = fmt.Sprintf("http://localhost:8010/%s", link)
			link = strings.TrimRight(link, "\r")

			res, err := http.Get(link) //Выполняется GET-запрос на сервер

			if err != nil {
				log.Fatal(err)
			}

			answer, err := io.ReadAll(res.Body) //Читается тело ответа сервера

			if err != nil {
				continue
			}

			fmt.Println(string(answer)) //Выводится исходная ссылка

			defer res.Body.Close()
			break

		case "3":
			return
		default:
			fmt.Println("Неправильный ввод")
		}
	}
}
