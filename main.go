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

type Device struct {
	Name string
	IP   string
	IsOn bool
}

const (
	ACME_SERVER_URL           = "http://localhost:8080/cse-in"
	APPLICATION_ENTITY_NAME   = "Smart-Switch"
	CONTAINER_NAME            = "Status"
	ORIGINATOR                = "CAdmin2"
	TARGET_APPLICATION_ENTITY = "Light-Bulb"
	TARGET_CONTAINER          = "Is-On"
)

type ServiceInfo struct {
	IP   string
	Port int
}

// discoverServices browses the network for mDNS services and returns a list of IP and port
func discoverServices(serviceType, domain string) ([]ServiceInfo, error) {
	var services []ServiceInfo

	// Create a resolver instance
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize resolver: %v", err)
	}

	// Create a channel to receive results
	entries := make(chan *zeroconf.ServiceEntry)

	// Context timeout for mDNS browsing
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	// Start mDNS service browsing
	go func() {
		err := resolver.Browse(ctx, serviceType, domain, entries)
		if err != nil {
			log.Println("Failed to browse mDNS services:", err)
			return
		}
	}()

	// Process entries and collect IP & Port
	for entry := range entries {
		// For each found service, append the IP and Port to the result slice
		for _, ip := range entry.AddrIPv4 {
			services = append(services, ServiceInfo{
				IP:   ip.String(),
				Port: entry.Port,
			})
		}
	}

	// Return the list of services found
	return services, nil
}

func checkApplicationEntityExists() bool {

	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s?fu=1&ty=2", ACME_SERVER_URL), nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("X-M2M-Origin", ORIGINATOR)
	req.Header.Set("X-M2M-RI", "123")
	req.Header.Set("X-M2M-RVI", "3")
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
	if strings.Contains(string(bodyText), APPLICATION_ENTITY_NAME) {
		return true
	}
	return false
}

func createApplicationEntityRequest() bool {

	client := &http.Client{}
	var data = strings.NewReader(fmt.Sprintf(`{"m2m:ae": {"rn": "%s", "api":"NnotebookAE", "rr": true, "srv": ["3"]}}`, APPLICATION_ENTITY_NAME))
	req, err := http.NewRequest("POST", ACME_SERVER_URL, data)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("X-M2M-Origin", ORIGINATOR)
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

	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		return true
	}

	return false
}

func createContainerRequest() bool {

	client := &http.Client{}
	var data = strings.NewReader(fmt.Sprintf(`{"m2m:cnt": {"rn" : "%s"}}`, CONTAINER_NAME))
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s", ACME_SERVER_URL, APPLICATION_ENTITY_NAME), data)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("X-M2M-Origin", ORIGINATOR)
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

	if resp.StatusCode == 200 || resp.StatusCode == 201 || resp.StatusCode == 409 {
		return true
	}

	return false
}

func changeStateRequest(targetURL string, state *bool) bool {

	client := &http.Client{}
	*state = !*state
	var data = strings.NewReader(fmt.Sprintf(`{"m2m:cin":{"con": "%t", "cnf": "text/plain:0"}}`, *state))
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s/%s", targetURL, TARGET_APPLICATION_ENTITY, TARGET_CONTAINER), data)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("X-M2M-Origin", ORIGINATOR)
	req.Header.Set("X-M2M-RI", "123")
	req.Header.Set("X-M2M-RVI", "3")
	req.Header.Set("Content-Type", "application/json;ty=4")
	req.Header.Set("Accept", "application/json")
	//fmt.Printf("%s", req)
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

func getContentInstance(targetURL string, content *bool) bool {
	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s/%s/la", targetURL, TARGET_APPLICATION_ENTITY, TARGET_CONTAINER), nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("X-M2M-Origin", ORIGINATOR)
	req.Header.Set("X-M2M-RI", "123")
	req.Header.Set("X-M2M-RVI", "3")
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

	// Criando um mapa genérico para fazer o parse
	var result map[string]map[string]interface{}

	err = json.Unmarshal(bodyText, &result)
	if err != nil {
		fmt.Println("Erro ao fazer unmarshal:", err)
		return false
	}

	// Acessando o valor de "con"
	conVal, ok := result["m2m:cin"]["con"]
	if ok {
		fmt.Println("Valor de 'con':", conVal)
		if conVal == "true" || conVal == true || conVal == "True" {
			*content = true
		} else {
			*content = false
		}
	} else {
		fmt.Println("'con' não encontrado.")
	}

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

	var services []ServiceInfo
	var err error

	go func() {
		// Create a application entity
		appendLog(log, "Verificando se a entidade de aplicação já existe...")
		if !checkApplicationEntityExists() {
			appendLog(log, "Entidade de aplicação não existe.")
			appendLog(log, "Inicializando entidade de aplicação...")
			if !createApplicationEntityRequest() {
				showErrorDialog(window, myApp, "Clique em OK para fechar o aplicativo.")
				return
			}
		} else {
			appendLog(log, "Entidade de aplicação já existe.")
		}

		// Create a container
		appendLog(log, "Entidade de aplicação criada com sucesso.")
		appendLog(log, "Criando contêiner...")
		if !createContainerRequest() {
			showErrorDialog(window, myApp, "Clique em OK para fechar o aplicativo.")
			return
		}
		appendLog(log, "Contêiner criado com sucesso.")

		// Discover services
		appendLog(log, "Descobrindo serviços...")
		services, err = discoverServices("_http._tcp", "local.")
		if err != nil {
			showErrorDialog(window, myApp, "Erro ao descobrir serviços: "+err.Error())
			return
		}
		if len(services) == 0 {
			showErrorDialog(window, myApp, "Nenhum serviço encontrado.")
			return
		}
		appendLog(log, fmt.Sprintf("Serviços encontrados: %d", len(services)))
		for _, service := range services {
			appendLog(log, fmt.Sprintf("Serviço encontrado: %s:%d", service.IP, service.Port))
		}
	}()

	// Lista de dispositivos
	devices := []Device{
		{"Dispositivo A", fmt.Sprintf("%s/%s", services[0].IP, services[0].Port), false},
		//{"Dispositivo B", "192.168.1.11", false},
		//{"Dispositivo C", "192.168.1.12", false},
	}

	for i := 0; i < len(devices); i++ {
		getContentInstance(devices[i].IP, &devices[i].IsOn)
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
			label := widget.NewLabel(fmt.Sprintf("Nome: %s | IP: %s | is On: %t", d.Name, d.IP, d.IsOn))
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
		changeStateRequest(devices[selectedIndex].IP, &devices[selectedIndex].IsOn)
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
