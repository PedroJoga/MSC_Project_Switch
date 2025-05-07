package main

import (
	"fmt"
	"image/color"
	"io"
	"log"
	"net/http"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type Device struct {
	Name   string
	IP     string
	Status string
}

const (
	CSE_BASE_URL            = "http://localhost:8080/cse-in"
	CSE_RESOURCE_NAME       = "in-cse"
	AE_RESOURCE_NAME        = "Switch-AE"
	AE_API_ID               = "NSwitchAE"
	CONTAINER_RESOURCE_NAME = "SwitchContainer"
	ORIGINATOR              = "CAdmin"
	VERSION                 = "1"
)

func createApplicationRequest() bool {

	client := &http.Client{}
	var data = strings.NewReader(`{"m2m:ae": {"rn": "Notebook-AE", "api":"NnotebookAE", "rr": true, "srv": ["3"]}}`)
	req, err := http.NewRequest("POST", "http://localhost:8080/cse-in", data)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("X-M2M-Origin", "CAdmin4")
	req.Header.Set("X-M2M-RI", "123")
	req.Header.Set("X-M2M-RVI", "3")
	req.Header.Set("Content-Type", "application/json;ty=2")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Status code: %s, %s\n", resp.Status, bodyText)

	if resp.StatusCode == 409 {
		return true
	}

	return false
}

func createContainerRequest() bool {

	client := &http.Client{}
	var data = strings.NewReader(`{"m2m:cnt": {"rn" : "CONT_1"}}`)
	req, err := http.NewRequest("POST", "http://localhost:8080/cse-in/Notebook-AE", data)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("X-M2M-Origin", "CAdmin2")
	req.Header.Set("X-M2M-RI", "123")
	req.Header.Set("X-M2M-RVI", "3")
	req.Header.Set("Content-Type", "application/json;ty=3")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Status code: %s, %s\n", resp.Status, bodyText)

	if resp.StatusCode == 409 {
		return true
	}

	return false
}

func changeStateRequest() bool {

	client := &http.Client{}
	var data = strings.NewReader(`{"m2m:cin":{"con": "abc", "cnf": "text/plain:0"}}`)
	req, err := http.NewRequest("POST", "http://localhost:8080/cse-in/Notebook-AE/CONT_1", data)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("X-M2M-Origin", "CAdmin2")
	req.Header.Set("X-M2M-RI", "123")
	req.Header.Set("X-M2M-RVI", "3")
	req.Header.Set("Content-Type", "application/json;ty=4")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Status code: %s, %s\n", resp.Status, bodyText)

	return false
}

func showErrorDialog(win fyne.Window, app fyne.App, message string) {
	dialog.ShowCustomConfirm(
		"Error",
		"OK",
		"",
		widget.NewLabel(message),
		func(confirm bool) {
			if confirm {
				app.Quit()
			}
		},
		win,
	)
}

func appendLog(log *widget.Entry, msg string) {
	log.SetText(log.Text + msg + "\n")
}

func main() {
	myApp := app.New()
	window := myApp.NewWindow("Registro Switch AE/Container")
	window.Resize(fyne.NewSize(600, 400))

	log := widget.NewMultiLineEntry()
	log.SetMinRowsVisible(10)
	//log.ReadOnly()

	go func() {
		appendLog(log, "Inicializando entidade de aplicação...")
		if !createApplicationRequest() {
			showErrorDialog(window, myApp, "Clique em OK para fechar o aplicativo.")
			return
		}
		appendLog(log, "Entidade de aplicação criada com sucesso.")
		appendLog(log, "Criando contêiner...")
		if !createContainerRequest() {
			showErrorDialog(window, myApp, "Clique em OK para fechar o aplicativo.")
			return
		}
		appendLog(log, "Contêiner criado com sucesso.")
	}()

	// Lista de dispositivos
	devices := []Device{
		{"Dispositivo A", "192.168.1.10", "Online"},
		{"Dispositivo B", "192.168.1.11", "Offline"},
		{"Dispositivo C", "192.168.1.12", "Online"},
	}

	// Índice do dispositivo selecionado
	selectedIndex := 0

	// Widget para exibir a lista visualmente
	var deviceBoxes []*fyne.Container
	devicesList := container.NewVBox()

	updateDeviceList := func() {
		devicesList.Objects = nil // limpa a lista visual
		deviceBoxes = []*fyne.Container{}
		for i, d := range devices {
			label := widget.NewLabel(fmt.Sprintf("Nome: %s | IP: %s | Status: %s", d.Name, d.IP, d.Status))
			bg := canvas.NewRectangle(color.RGBA{95, 95, 95, 160})
			if i == selectedIndex {
				bg.FillColor = color.RGBA{0, 0, 0, 255}
			}
			box := container.NewStack(bg, label)
			deviceBoxes = append(deviceBoxes, box)
			devicesList.Add(box)
		}
		devicesList.Refresh()
	}

	updateDeviceList()

	switchButton := widget.NewButton("Trocar Destaque", func() {
		selectedIndex = (selectedIndex + 1) % len(devices)
		updateDeviceList()
		appendLog(log, fmt.Sprintf("Dispositivo selecionado: %s", devices[selectedIndex].Name))
	})

	actionButton := widget.NewButton("Executar Ação", func() {
		changeStateRequest()
		appendLog(log, "Função ainda não implementada chamada.")
	})

	content := container.NewVBox(
		devicesList,
		switchButton,
		actionButton,
		log,
	)

	window.SetContent(content)
	window.Show()

	myApp.Run()
}
