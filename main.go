package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"gopkg.in/tucnak/telebot.v2"
)

type TranscriptionResponse struct {
	Text string `json:"text"`
}

func main() {

	telegramToken := os.Getenv("TELEGRAM_TOKEN")
	hfToken := os.Getenv("HF_TOKEN")

	bot, err := telebot.NewBot(telebot.Settings{
		Token:  telegramToken,
		Poller: &telebot.LongPoller{Timeout: 10},
	})
	if err != nil {
		fmt.Println("Failed to create bot:", err)
		return
	}

	log.Println("🚀 Bot started successfully!")

	bot.Handle("/start", func(m *telebot.Message) {
		bot.Send(m.Chat, "Hello, I am a transcribator3000")
	})

	bot.Handle(telebot.OnVoice, func(m *telebot.Message) {
		chat := m.Chat

		fileURL, err := bot.FileURLByID(m.Voice.FileID)
		if err != nil {
			bot.Send(chat, "Error getting file link")
			return
		}

		filePath, err := downloadFile(fileURL)
		if err != nil {
			bot.Send(chat, "Error downloading file")
			return
		}
		defer os.Remove(filePath)

		transcription, err := transcribeAudio(filePath, hfToken)
		if err != nil {
			bot.Send(chat, "Error processing transcription")
			return
		}

		bot.Send(chat, transcription, &telebot.SendOptions{ReplyTo: m})
	})

	bot.Start()
}

func downloadFile(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	fileType := getFileTypeByUrl(url)
	randBytes := make([]byte, 16)
	if _, err := rand.Read(randBytes); err != nil {
		return "", err
	}
	fileName := fmt.Sprintf("./temp/%s.%s", hex.EncodeToString(randBytes), fileType)

	outFile, err := os.Create(fileName)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return "", err
	}
	return fileName, nil
}

func getFileTypeByUrl(url string) string {
	parts := strings.Split(url, "/")
	file := parts[len(parts)-1]
	fileParts := strings.Split(file, ".")
	if len(fileParts) > 1 {
		return fileParts[len(fileParts)-1]
	}
	return "wav"
}

func transcribeAudio(filePath string, token string) (string, error) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api-inference.huggingface.co/models/openai/whisper-large-v3-turbo", bytes.NewReader(fileData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "audio/wav")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("failed to get transcription")
	}

	var result TranscriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Text, nil
}
