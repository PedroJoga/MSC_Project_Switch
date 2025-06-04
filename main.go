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
	"fyne.io/fyne/v2/dialog"
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

func findServices(updateCallback func(ServiceInfo), finishCallback func()) {
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
		log.Println("Finished browsing for services")
		finishCallback()
		cancel() // Clean up when done
	}()

	// Start discovery (non-blocking)
	go func() {
		log.Println("Browsing for services...")
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

func serviceExists(services []ServiceInfo, newService ServiceInfo) bool {
	for _, service := range services {
		if service.Name == newService.Name {
			return true
		}
	}
	return false
}

func main() {
	myApp := app.New()
	window := myApp.NewWindow("Registro Switch AE/Container")
	window.Resize(fyne.NewSize(600, 400))

	var init time.Time // for metrics

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
		init = time.Now()
		foundservices := []ServiceInfo{} // Clear existing
		updateDeviceList()
		appendLog("Procurando dispositivos...")

		// Start non-blocking discovery with dynamic updates
		findServices(func(service ServiceInfo) {
			// This callback runs for each discovered service
			appendLog(fmt.Sprintf("Dispositivo encontrado: %s (%s:%d) (%d ms)", service.Name, service.IP, service.Port, time.Since(init).Milliseconds()))
			if !serviceExists(services, service) {
				services = append(services, service)
			}
			foundservices = append(foundservices, service)
			updateDeviceList() // Update UI immediately
		}, func() { // Finish callback
			log.Printf("Found %d services", len(foundservices))
			services = []ServiceInfo{}
			services = foundservices
			updateDeviceList()
		})
	}

	// Search for devices every 10 seconds
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			findDevices()
		}
	}()

	// Initialize application entity and container
	go func() {
		init = time.Now()
		appendLog("Verificando se a entidade de aplicação já existe...")
		if !checkApplicationEntityExists() {
			appendLog(fmt.Sprintf("Entidade de aplicação não existe. (%dms)", time.Since(init).Milliseconds()))
			appendLog("Inicializando entidade de aplicação...")
			if !createApplicationEntityRequest() {
				showErrorDialog(window, myApp, fmt.Sprintf("Falha ao criar entidade de aplicação. Clique em OK para fechar. (%dms)", time.Since(init).Milliseconds()))
				return
			}
		} else {
			appendLog(fmt.Sprintf("Entidade de aplicação já existe. (%dms)", time.Since(init).Milliseconds()))
		}

		appendLog("Criando contêiner...")
		init = time.Now()
		if !createContainerRequest() {
			showErrorDialog(window, myApp, fmt.Sprintf("Falha ao criar contêiner. Clique em OK para fechar. (%dms)", time.Since(init).Milliseconds()))
			return
		}
		appendLog(fmt.Sprintf("Contêiner criado com sucesso. (%dms)", time.Since(init).Milliseconds()))
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
		init = time.Now()
		if changeStateRequest(services[selectedIndex].getAddress(), &services[selectedIndex].IsOn) {
			appendLog(fmt.Sprintf("Ação executada com sucesso (%dms)", time.Since(init).Milliseconds()))
		} else {
			appendLog(fmt.Sprintf("Falha ao executar ação (%dms)", time.Since(init).Milliseconds()))
		}
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
