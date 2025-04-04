package main

import (
	"bytes"
	"net/http"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func makeRequest(statusLabel *widget.Label) {
	url := "http://localhost:8080/cse-in"
	jsonData := []byte(`{
		"m2m:ae": {
			"rn": "Notebook-AE",
			"api": "NnotebookAE",
			"rr": true,
			"srv": ["3"]
		}
	}`)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		statusLabel.SetText("Erro na req")
		return
	}

	req.Header.Set("X-M2M-Origin", "CAdmin3")
	req.Header.Set("X-M2M-RI", "123")
	req.Header.Set("X-M2M-RVI", "3")
	req.Header.Set("Content-Type", "application/json;ty=2")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		statusLabel.SetText("Falha na req")
		return
	}
	defer resp.Body.Close()

	statusLabel.SetText(resp.Status)
}

func main() {
	a := app.New()
	w := a.NewWindow("Req")
	w.Resize(fyne.NewSize(100, 100))

	statusLabel := widget.NewLabel("Status")
	button := widget.NewButton("Enviar", func() {
		statusLabel.SetText("Enviando...")
		go makeRequest(statusLabel)
	})

	w.SetContent(container.NewVBox(button, statusLabel))
	w.ShowAndRun()
}
