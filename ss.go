package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
)

type staticNote struct {
	ipsrc    string
	longUrl  string
	shortUrl string
	time     string
}

func (s *staticNote) get(field string) interface{} {
	if field == "SourceIP" {
		return s.ipsrc
	}
	if field == "URL" {
		return fmt.Sprintf("%s (%s)", s.longUrl, s.shortUrl)
	}
	if field == "TimeInterval" {
		//2023-12-06T02:40:47+07:00
		startTime := s.time[11:16]
		min, err := strconv.Atoi(s.time[14:16])
		if err != nil {
			return nil
		}
		min += 1
		endTime := s.time[11:14]
		endTime += strconv.Itoa(min)

		return fmt.Sprintf("%s - %s", startTime, endTime)
	}
	return nil
}

type reportNote struct {
	Id           interface{}
	Pid          interface{}
	SourceIP     interface{}
	URL          interface{}
	TimeInterval interface{}
	Count        int
}

func (r *reportNote) set(field string, value interface{}) {
	switch field {
	case "SourceIP":
		r.SourceIP = value
	case "URL":
		r.URL = value
	case "TimeInterval":
		r.TimeInterval = value
	case "Id":
		r.Id = value
	case "count":
		r.Count = value.(int)
	}
}

func (r *reportNote) equals(other *reportNote) bool {
	return r.SourceIP == other.SourceIP && r.URL == other.URL && r.TimeInterval == other.TimeInterval && r.Pid == other.Pid
}

type StatisticRequestHandler struct {
	staticNotes []staticNote
}

func (h *StatisticRequestHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/":
		if req.Method == http.MethodPost {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			var note staticNote
			//note.shortUrl = myMap["shortUrl"]

			//Time:=strconv.FormatFloat(myMap["time"], 'f', -1, 64)
			myMap := make(map[string]string)
			err = json.Unmarshal(body, &myMap)
			log.Println("Mymap:", myMap)
			note.ipsrc = myMap["ipsrc"]
			note.longUrl = myMap["url"]
			note.time = myMap["time"]
			note.shortUrl = myMap["shortUrl"]
			log.Println("note:", note)

			h.staticNotes = append(h.staticNotes, note)
			w.WriteHeader(http.StatusOK)
			return
		}

	case "/report":
		if req.Method == http.MethodPost {
			logs := h.staticNotes
			log.Println("Logs", logs)
			log.Printf("Case /report: %s", logs)

			body, err := io.ReadAll(req.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			var clientReq []string
			err = json.Unmarshal(body, &clientReq)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			report := make([]reportNote, 0)
			pids := make(map[interface{}]interface{})
			id := 1

			for _, l := range logs {
				record := reportNote{Count: 1}
				record.set(clientReq[0], l.get(clientReq[0]))
				record.set("Id", id)
				if index := indexOf(report, &record); index != -1 {
					report[index].Count++
				} else {
					report = append(report, record)
					pids[l.get(clientReq[0])] = id
					id++
				}

				record = reportNote{Count: 1}
				record.set(clientReq[1], l.get(clientReq[1]))
				pid := pids[l.get(clientReq[0])]
				record.Pid = pid
				if index := indexOf(report, &record); index != -1 {
					report[index].Count++
				} else {
					pids[l.get(clientReq[1])] = id
					record.set("Id", id)
					report = append(report, record)
					id++
				}

				record = reportNote{Count: 1}
				record.set(clientReq[2], l.get(clientReq[2]))
				pid = pids[l.get(clientReq[1])]
				record.Pid = pid
				if index := indexOf(report, &record); index != -1 {
					report[index].Count++
				} else {
					pids[l.get(clientReq[2])] = id
					record.set("Id", id)
					report = append(report, record)
					id++
				}
			}

			answer, err := json.Marshal(report)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write(answer)
			return
		}

	default:
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found file"))
	}
}

func indexOf(arr []reportNote, target *reportNote) int {
	for i, item := range arr {
		if item.equals(target) {
			return i
		}
	}
	return -1
}

func main() {
	handler := &StatisticRequestHandler{}
	server := &http.Server{
		Addr:    "127.0.0.1:8080",
		Handler: handler,
	}
	fmt.Println("Server running on 127.0.0.1:8080")
	log.Fatal(server.ListenAndServe())
}
