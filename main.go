package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/grandcat/zeroconf"
)

const (
	ACME_SERVER_URL           = "http://localhost:8080/cse-in"
	APPLICATION_ENTITY_NAME   = "Smart-Switch"
	CONTAINER_NAME            = "Status"
	ORIGINATOR                = "CAdmin2"
	TARGET_APPLICATION_ENTITY = "Light-Bulb"
	TARGET_CONTAINER          = "Is-On"
)

type ServiceInfo struct {
	Name string
	IP   string
	Port int
	IsOn bool
}

func (service ServiceInfo) getAddress() string {
	return fmt.Sprintf("http://%s:%d/cse-in", service.IP, service.Port)
}

func findServices(updateCallback func(ServiceInfo)) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Printf("Failed to create resolver: %v", err)
		return
	}

	entries := make(chan *zeroconf.ServiceEntry)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)

	// Process entries and call update callback for each one
	go func() {
		for entry := range entries {
			for _, ip := range entry.AddrIPv4 {
				service := ServiceInfo{
					Name: entry.Instance,
					IP:   ip.String(),
					Port: entry.Port,
				}
				// Get the service state
				getContentInstance(service.getAddress(), &service.IsOn)
				// Call the update callback
				updateCallback(service)
			}
		}
		cancel() // Clean up when done
	}()

	// Start discovery (non-blocking)
	go func() {
		resolver.Browse(ctx, "_http._tcp", "local.", entries)
	}()
}

func checkApplicationEntityExists() bool {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s?fu=1&ty=2", ACME_SERVER_URL), nil)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return false
	}
	req.Header.Set("X-M2M-Origin", ORIGINATOR)
	req.Header.Set("X-M2M-RI", "123")
	req.Header.Set("X-M2M-RVI", "3")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error checking application entity: %v", err)
		return false
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response: %v", err)
		return false
	}
	return strings.Contains(string(bodyText), APPLICATION_ENTITY_NAME)
}

func createApplicationEntityRequest() bool {
	client := &http.Client{Timeout: 5 * time.Second}
	data := strings.NewReader(fmt.Sprintf(`{"m2m:ae": {"rn": "%s", "api":"NnotebookAE", "rr": true, "srv": ["3"]}}`, APPLICATION_ENTITY_NAME))
	req, err := http.NewRequest("POST", ACME_SERVER_URL, data)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return false
	}
	req.Header.Set("X-M2M-Origin", ORIGINATOR)
	req.Header.Set("X-M2M-RI", "123")
	req.Header.Set("X-M2M-RVI", "3")
	req.Header.Set("Content-Type", "application/json;ty=2")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error creating application entity: %v", err)
		return resp.StatusCode == 403
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200 || resp.StatusCode == 201
}

func createContainerRequest() bool {
	client := &http.Client{Timeout: 5 * time.Second}
	data := strings.NewReader(fmt.Sprintf(`{"m2m:cnt": {"rn" : "%s"}}`, CONTAINER_NAME))
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s", ACME_SERVER_URL, APPLICATION_ENTITY_NAME), data)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return false
	}
	req.Header.Set("X-M2M-Origin", ORIGINATOR)
	req.Header.Set("X-M2M-RI", "123")
	req.Header.Set("X-M2M-RVI", "3")
	req.Header.Set("Content-Type", "application/json;ty=3")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error creating container: %v", err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200 || resp.StatusCode == 201 || resp.StatusCode == 409
}

func changeStateRequest(targetURL string, state *bool) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	*state = !*state
	data := strings.NewReader(fmt.Sprintf(`{"m2m:cin":{"con": %t, "cnf": "text/plain:0"}}`, *state))
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s/%s", targetURL, TARGET_APPLICATION_ENTITY, TARGET_CONTAINER), data)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return false
	}
	req.Header.Set("X-M2M-Origin", ORIGINATOR)
	req.Header.Set("X-M2M-RI", "123")
	req.Header.Set("X-M2M-RVI", "3")
	req.Header.Set("Content-Type", "application/json;ty=4")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error changing state: %v", err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200 || resp.StatusCode == 201
}

func getContentInstance(targetURL string, content *bool) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s/%s/la", targetURL, TARGET_APPLICATION_ENTITY, TARGET_CONTAINER), nil)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return false
	}
	req.Header.Set("X-M2M-Origin", ORIGINATOR)
	req.Header.Set("X-M2M-RI", "123")
	req.Header.Set("X-M2M-RVI", "3")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error getting content: %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false
	}

	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	var result map[string]map[string]interface{}
	err = json.Unmarshal(bodyText, &result)
	if err != nil {
		fmt.Println("Erro ao fazer unmarshal:", err)
		return false
	}

	// Check if the structure exists before accessing
	if cinData, exists := result["m2m:cin"]; exists {
		if conVal, ok := cinData["con"]; ok {
			//fmt.Println("Valor de 'con':", conVal)
			switch v := conVal.(type) {
			case bool:
				*content = v
			case string:
				*content = v == "true" || v == "True"
			default:
				*content = false
			}
			return true
		}
	}
	log.Println("'con' não encontrado.")
	return false
}

func main() {
	myApp := app.New()
	window := myApp.NewWindow("Registro Switch AE/Container")
	window.Resize(fyne.NewSize(600, 400))

	logWidget := widget.NewMultiLineEntry()
	logWidget.SetMinRowsVisible(8)
	logWidget.Wrapping = fyne.TextWrapWord

	var services []ServiceInfo
	selectedIndex := 0

	var deviceBoxes []*fyne.Container
	devicesList := container.NewVBox()

	appendLog := func(msg string) {
		logWidget.SetText(logWidget.Text + msg + "\n")
	}

	updateDeviceList := func() {
		devicesList.Objects = nil
		deviceBoxes = []*fyne.Container{}
		for i, d := range services {
			label := widget.NewLabel(fmt.Sprintf("Nome: %s | IP: %s:%d | is On: %t", d.Name, d.IP, d.Port, d.IsOn))
			bg := canvas.NewRectangle(color.RGBA{200, 200, 200, 100})
			if i == selectedIndex {
				bg.FillColor = color.RGBA{100, 150, 255, 200}
			}
			box := container.NewStack(bg, label)
			deviceBoxes = append(deviceBoxes, box)
			devicesList.Add(box)
		}
		devicesList.Refresh()
	}

	findDevices := func() {
		services = []ServiceInfo{} // Clear existing
		updateDeviceList()
		appendLog("Procurando dispositivos...")

		// Start non-blocking discovery with dynamic updates
		findServices(func(service ServiceInfo) {
			// This callback runs for each discovered service
			appendLog(fmt.Sprintf("Dispositivo encontrado: %s (%s:%d)", service.Name, service.IP, service.Port))
			services = append(services, service)
			updateDeviceList() // Update UI immediately
		})
	}

	// Initialize application entity and container
	go func() {
		appendLog("Verificando se a entidade de aplicação já existe...")
		if !checkApplicationEntityExists() {
			appendLog("Entidade de aplicação não existe.")
			appendLog("Inicializando entidade de aplicação...")
			if !createApplicationEntityRequest() {
				appendLog("Falha ao criar entidade de aplicação.")
				return
			}
		} else {
			appendLog("Entidade de aplicação já existe.")
		}

		appendLog("Criando contêiner...")
		if !createContainerRequest() {
			appendLog("Falha ao criar contêiner.")
			return
		}
		appendLog("Contêiner criado com sucesso.")

		// Auto-discover devices on startup
		findDevices()
	}()

	updateDeviceList()

	findButton := widget.NewButton("Procurar dispositivos", func() {
		findDevices()
	})

	switchButton := widget.NewButton("Trocar Destaque", func() {
		if len(services) <= 0 {
			appendLog("Nenhum serviço na lista")
			return
		}

		selectedIndex = (selectedIndex + 1) % len(services)
		updateDeviceList()
		appendLog(fmt.Sprintf("Dispositivo selecionado: %s", services[selectedIndex].Name))
	})

	actionButton := widget.NewButton("Executar Ação", func() {
		if len(services) <= 0 {
			appendLog("Nenhum serviço na lista")
			return
		}

		appendLog(fmt.Sprintf("Executando ação no dispositivo: %s", services[selectedIndex].Name))
		changeStateRequest(services[selectedIndex].getAddress(), &services[selectedIndex].IsOn)
		updateDeviceList()
	})

	content := container.NewVBox(
		widget.NewLabel("Dispositivos Descobertos:"),
		devicesList,
		container.NewHBox(findButton, switchButton, actionButton),
		widget.NewSeparator(),
		widget.NewLabel("Log:"),
		logWidget,
	)

	window.SetContent(content)
	window.ShowAndRun()
}
