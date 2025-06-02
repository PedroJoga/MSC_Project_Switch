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
	"sync"
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

func discoverServices(serviceType, domain string, servicesChannel chan ServiceInfo) error {
	// Create a resolver instance
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		close(servicesChannel)
		return fmt.Errorf("failed to initialize resolver: %v", err)
	}

	// Create a channel to receive results from zeroconf
	entries := make(chan *zeroconf.ServiceEntry, 10) // Buffered to prevent blocking

	// Context timeout for mDNS browsing
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*8)

	// zeroConf might try to close channel at the same time as us so we should do it safely
	var channelClosed bool
	var mu sync.Mutex

	safeCloseChannel := func() {
		mu.Lock()
		defer mu.Unlock()
		if !channelClosed {
			close(servicesChannel)
			channelClosed = true
		}
	}

	// Process entries and send them to servicesChannel
	go func(results <-chan *zeroconf.ServiceEntry) {
		defer safeCloseChannel()
		for entry := range results {
			log.Printf("Found service: %s at port %d", entry.Instance, entry.Port)
			for _, ip := range entry.AddrIPv4 {
				service := ServiceInfo{
					Name: entry.Instance,
					IP:   ip.String(),
					Port: entry.Port,
				}
				// Send each discovered service immediately
				servicesChannel <- service
			}
		}
		log.Println("Service discovery completed.")
	}(entries)

	// Start mDNS service browsing in a goroutine to avoid blocking
	go func() {
		defer func() {
			// Handle any panics from the zeroconf library
			if r := recover(); r != nil {
				log.Printf("Recovered from zeroconf panic: %v", r)
				safeCloseChannel()
			}
		}()

		err := resolver.Browse(ctx, serviceType, domain, entries)
		if err != nil {
			log.Printf("Browse error: %v", err)
		}
	}()

	// Wait for context to be done, then cleanup
	go func() {
		<-ctx.Done()
		cancel() // Cancel the context

		// Give a moment for cleanup, then force close if needed
		time.Sleep(500 * time.Millisecond)

		// Close entries channel safely
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered from channel close panic: %v", r)
			}
		}()

		// Try to close entries channel - might already be closed by zeroconf
		select {
		case <-entries:
			// Channel already closed
		default:
			close(entries)
		}

		safeCloseChannel()
	}()

	return nil
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
	fmt.Printf("Status code: %s, %s\n", resp.Status, bodyText)
	return strings.Contains(string(bodyText), APPLICATION_ENTITY_NAME)
}

func createApplicationEntityRequest() bool {
	client := &http.Client{Timeout: 5 * time.Second}
	var data = strings.NewReader(fmt.Sprintf(`{"m2m:ae": {"rn": "%s", "api":"NnotebookAE", "rr": true, "srv": ["3"]}}`, APPLICATION_ENTITY_NAME))
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
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response: %v", err)
		return false
	}
	fmt.Printf("Status code: %s, %s\n", resp.Status, bodyText)

	return resp.StatusCode == 200 || resp.StatusCode == 201
}

func createContainerRequest() bool {
	client := &http.Client{Timeout: 5 * time.Second}
	var data = strings.NewReader(fmt.Sprintf(`{"m2m:cnt": {"rn" : "%s"}}`, CONTAINER_NAME))
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
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response: %v", err)
		return false
	}
	fmt.Printf("Status code: %s, %s\n", resp.Status, bodyText)

	return resp.StatusCode == 200 || resp.StatusCode == 201 || resp.StatusCode == 409
}

func changeStateRequest(targetURL string, state *bool) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	*state = !*state
	var data = strings.NewReader(fmt.Sprintf(`{"m2m:cin":{"con": %t, "cnf": "text/plain:0"}}`, *state))
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
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response: %v", err)
		return false
	}
	fmt.Printf("Status code: %s, %s\n", resp.Status, bodyText)
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
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response: %v", err)
		return false
	}

	fmt.Printf("Status code: %s, %s\n", resp.Status, bodyText)

	// Only process if we got a successful response
	if resp.StatusCode != 200 {
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
			fmt.Println("Valor de 'con':", conVal)
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

	fmt.Println("'con' não encontrado.")
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
	window.Resize(fyne.NewSize(700, 500))

	logWidget := widget.NewMultiLineEntry()
	logWidget.SetMinRowsVisible(8)
	logWidget.Wrapping = fyne.TextWrapWord

	var services []ServiceInfo
	selectedIndex := 0

	var deviceBoxes []*fyne.Container
	devicesList := container.NewVBox()

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
		services = []ServiceInfo{} // Clear existing services
		updateDeviceList()

		appendLog(logWidget, "Procurando dispositivos...")

		// Create a new channel for each discovery
		servicesChannel := make(chan ServiceInfo, 10) // Buffered channel

		// Start service discovery
		go func() {
			err := discoverServices("_http._tcp", "local.", servicesChannel)
			if err != nil {
				appendLog(logWidget, fmt.Sprintf("Erro na descoberta: %v", err))
				return
			}
		}()

		// Handle incoming services
		go func() {
			deviceCount := 0
			discoveryTimeout := time.After(10 * time.Second) // Reduced timeout to match discovery

			for {
				select {
				case service, ok := <-servicesChannel:
					if !ok {
						// Channel closed, discovery completed
						if deviceCount == 0 {
							appendLog(logWidget, "Nenhum dispositivo encontrado")
						} else {
							appendLog(logWidget, fmt.Sprintf("Descoberta concluída. Total: %d dispositivos", deviceCount))
						}
						return
					}

					deviceCount++

					// Filter for LAMP services or relevant services
					if strings.Contains(strings.ToLower(service.Name), "lamp") || service.Port == 8081 {
						// Get the current state of the device
						getContentInstance(service.getAddress(), &service.IsOn)

						// Add to services list
						services = append(services, service)

						// Update UI
						appendLog(logWidget, fmt.Sprintf("Dispositivo %d encontrado: %s (%s:%d)", deviceCount, service.Name, service.IP, service.Port))
						updateDeviceList()

						log.Printf("Service %d: Name: %s, IP: %s, Port: %d\n", deviceCount, service.Name, service.IP, service.Port)
					} else {
						appendLog(logWidget, fmt.Sprintf("Serviço ignorado: %s (%s:%d)", service.Name, service.IP, service.Port))
					}

				case <-discoveryTimeout:
					// Timeout reached
					if deviceCount == 0 {
						appendLog(logWidget, "Timeout: Nenhum dispositivo encontrado")
					} else {
						appendLog(logWidget, fmt.Sprintf("Timeout: Descoberta concluída. Total: %d dispositivos", deviceCount))
					}
					return
				}
			}
		}()
	}

	// Initialize application entity and container
	go func() {
		appendLog(logWidget, "Verificando se a entidade de aplicação já existe...")
		if !checkApplicationEntityExists() {
			appendLog(logWidget, "Entidade de aplicação não existe.")
			appendLog(logWidget, "Inicializando entidade de aplicação...")
			if !createApplicationEntityRequest() {
				showErrorDialog(window, myApp, "Falha ao criar entidade de aplicação. Clique em OK para fechar.")
				return
			}
		} else {
			appendLog(logWidget, "Entidade de aplicação já existe.")
		}

		appendLog(logWidget, "Criando contêiner...")
		if !createContainerRequest() {
			showErrorDialog(window, myApp, "Falha ao criar contêiner. Clique em OK para fechar.")
			return
		}
		appendLog(logWidget, "Contêiner criado com sucesso.")

		// Auto-discover devices on startup
		findDevices()
	}()

	updateDeviceList()

	findButton := widget.NewButton("Procurar dispositivos", func() {
		findDevices()
	})

	switchButton := widget.NewButton("Trocar Destaque", func() {
		if len(services) <= 0 {
			showErrorDialog(window, myApp, "Nenhum serviço na lista")
			return
		}

		selectedIndex = (selectedIndex + 1) % len(services)
		updateDeviceList()
		appendLog(logWidget, fmt.Sprintf("Dispositivo selecionado: %s", services[selectedIndex].Name))
	})

	actionButton := widget.NewButton("Executar Ação", func() {
		if len(services) <= 0 {
			showErrorDialog(window, myApp, "Nenhum serviço na lista")
			return
		}

		appendLog(logWidget, fmt.Sprintf("Executando ação no dispositivo: %s", services[selectedIndex].Name))
		if changeStateRequest(services[selectedIndex].getAddress(), &services[selectedIndex].IsOn) {
			appendLog(logWidget, "Ação executada com sucesso")
		} else {
			appendLog(logWidget, "Falha ao executar ação")
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
	window.Show()

	myApp.Run()
}
