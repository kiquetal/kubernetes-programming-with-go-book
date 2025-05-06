package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"bufio"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	corev1 "k8s.io/api/core/v1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/rest"
)

// Implement the list.Item interface for displaying resources
type PodItem struct {
	Name string
}

func (p PodItem) FilterValue() string { return p.Name }
func (p PodItem) Title() string       { return p.Name }
func (p PodItem) Description() string { return "Pod" }

type DeploymentItem struct {
	Name string
}

func (d DeploymentItem) FilterValue() string { return d.Name }
func (d Item) Title() string       { return d.Name }
func (d DeploymentItem) Description() string { return "Deployment" }

type ServiceItem struct {
	Name string
}

func (s ServiceItem) FilterValue() string { return s.Name }
func (s ServiceItem) Title() string       { return s.Name }
func (s ServiceItem) Description() string { return "Service" }

// Model to manage the TUI
type model struct {
	podList        list.Model
	deploymentList list.Model
	serviceList    list.Model
	logView        viewport.Model
	logs           []string
	k8sClient      *kubernetes.Clientset
	krakendEndpoint string
	err            error
	selectedPod    string // Track the selected Pod for logs
}

func initialModel() model {

	podList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	podList.Title = "Pods (app=my-app)"

	deploymentList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	deploymentList.Title = "Deployments (app=my-app)"

	serviceList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	serviceList.Title = "Services (app=my-app)"

	return model{
		podList:        podList,
		deploymentList: deploymentList,
		serviceList:    serviceList,
		logView:        viewport.New(0, 0),
		logs:           []string{"Initializing..."},
		k8sClient:      nil, // Will be initialized in Init()
		krakendEndpoint: "Not fetched",
		err:            nil,
		selectedPod:    "",
	}
}

func (m model) Init() tea.Cmd {
	// Initialize client-go and fetch data
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: ""},
		&clientcmd.ConfigOverrides{
			Context: clientcmd.Config{ // Corrected
				Cluster:  "",
				AuthInfo: "",
				Namespace: "default", // set namespace
			},
		},
	).ClientConfig()
	if err != nil {
		//try in cluster config
		inClusterConfig, err := rest.InClusterConfig()
		if err != nil{
			return errorCmd(fmt.Errorf("error connecting to the client: %w", err))
		}
		config = inClusterConfig
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return errorCmd(fmt.Errorf("error creating clientset: %w", err))
	}
	m.k8sClient = clientset

	return tea.Batch(
		m.fetchPods(),
		m.fetchDeployments(),
		m.fetchServices(),
		m.fetchKrakendEndpoint(),
	) // Fetch data
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

		switch msg.String() {
		case "enter":
			// Handle selection to view logs
			selectedPodItem := m.podList.SelectedItem()
			if selectedPodItem != nil {
				m.selectedPod = selectedPodItem.(PodItem).Name
				m.logs = []string{}      // Clear previous logs
				m.logView.SetContent("") // Clear viewport
				return m, m.fetchPodLogs(m.selectedPod)
			}
			return m, nil

		case "esc":
			m.logs = []string{}
			m.logView.SetContent("")
			m.selectedPod = ""

		}

		// Update the lists
		var cmd tea.Cmd
		m.podList, cmd = m.podList.Update(msg)
		m.deploymentList, cmd = m.deploymentList.Update(msg)
		m.serviceList, cmd = m.serviceList.Update(msg)
		m.logView, cmd = m.logView.Update(msg)
		return m, cmd

	case podListMsg: // Custom message
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		// Update pod list
		var podItems []list.Item
		for _, pod := range msg.pods.Items {
			podItems = append(podItems, PodItem{Name: pod.Name})
		}
		m.podList.SetItems(podItems)
		return m, nil

	case deploymentListMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		deploymentItems := make([]list.Item, 0, len(msg.deployments.Items))
		for _, deployment := range msg.deployments.Items {
			deploymentItems = append(deploymentItems, DeploymentItem{Name: deployment.Name})
		}
		m.deploymentList.SetItems(deploymentItems)
		return m, nil

	case serviceListMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		serviceItems := make([]list.Item, 0, len(msg.services.Items))
		for _, service := range msg.services.Items {
			serviceItems = append(serviceItems, ServiceItem{Name: service.Name})
		}
		m.serviceList.SetItems(serviceItems)
		return m, nil

	case logUpdateMsg:
		m.logs = append(m.logs, msg.logLine)
		m.logView.SetContent(strings.Join(m.logs, "\n"))
		m.logView.GotoBottom()
		return m, nil

	case krakendEndpointMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.krakendEndpoint = msg.endpoint
		return m, nil

	case errorMsg:
		m.err = msg.err
		return m, nil

	default:
		return m, nil
	}
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\nPress Ctrl+C to quit.", m.err)
	}

	doc := strings.Builder{}

	// Title
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Render("Kubernetes & KrakenD Dashboard")
	doc.WriteString(title + "\n\n")

	// Grid layout
	podView := lipgloss.NewStyle().Width(30).Render(m.podList.View())
	deploymentView := lipgloss.NewStyle().Width(30).Render(m.deploymentList.View())
	serviceView := lipgloss.NewStyle().Width(30).Render(m.serviceList.View())
	logView := lipgloss.NewStyle().Width(60).Render(m.logView.View())
	krakendView := lipgloss.NewStyle().Width(60).Render(
		lipgloss.NewStyle().Padding(1, 0, 0, 0).Render(
			fmt.Sprintf("KrakenD Endpoint: %s", m.krakendEndpoint),
		),
	)

	// Top row: Pods, Deployments, Services
	row1 := lipgloss.JoinHorizontal(lipgloss.JoinStyle, podView, deploymentView, serviceView)
	doc.WriteString(row1 + "\n")

	// Bottom row: Logs, KrakenD
	row2 := lipgloss.JoinHorizontal(lipgloss.JoinStyle, logView, krakendView)
	doc.WriteString(row2 + "\n")

	// Status bar
	doc.WriteString("\n[↑/↓] Select | [Enter] View Logs | [ESC] Clear Logs | [Ctrl+C] Quit")

	return doc.String()
}

// Custom messages
type podListMsg struct {
	pods *corev1.PodList
	err  error
}

type deploymentListMsg struct {
	deployments *appsv1.DeploymentList
	err         error
}

type serviceListMsg struct {
	services *corev1.ServiceList
	err      error
}

type logUpdateMsg struct {
	logLine string
}

type krakendEndpointMsg struct {
	endpoint string
	err      error
}

type errorMsg struct {
	err error
}

func errorCmd(err error) tea.Cmd {
	return func() tea.Msg {
		return errorMsg{err: err}
	}
}

func (m model) fetchPods() tea.Cmd {
	return func() tea.Msg {
		pods, err := m.k8sClient.CoreV1().Pods("default").List(context.TODO(), metav1.ListOptions{LabelSelector: "app=my-app"})
		if err != nil {
			return podListMsg{err: err}
		}
		return podListMsg{pods: pods}
	}
}

func (m model) fetchDeployments() tea.Cmd {
	return func() tea.Msg {
		deployments, err := m.k8sClient.AppsV1().Deployments("default").List(context.TODO(), metav1.ListOptions{LabelSelector: "app=my-app"})
		if err != nil {
			return deploymentListMsg{err: err}
		}
		return deploymentListMsg{deployments: deployments}
	}
}

func (m model) fetchServices() tea.Cmd {
	return func() tea.Msg {
		services, err := m.k8sClient.CoreV1().Services("default").List(context.TODO(), metav1.ListOptions{LabelSelector: "app=my-app"})
		if err != nil {
			return serviceListMsg{err: err}
		}
		return serviceListMsg{services: services}
	}
}

func (m model) fetchPodLogs(podName string) tea.Cmd {
	return func() tea.Msg {
		req := m.k8sClient.CoreV1().Pods("default").GetLogs(podName, &corev1.PodLogOptions{Follow: true, TailLines: &[]int64{100}}) // Get last 100 lines
		readCloser, err := req.Stream(context.TODO())
		if err != nil {
			return errorMsg{err: fmt.Errorf("failed to get log stream for %s: %w", podName, err)}
		}

		go func() {
			defer readCloser.Close()
			reader := bufio.NewReader(readCloser)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					if err == io.EOF {
						return
					}
					log.Printf("Error reading log line: %v", err) // Important: Log the error within the goroutine
					return                                      // Stop reading, handle error as appropriate for your application
				}
				tea.Send(logUpdateMsg{logLine: line}) // send to чай
			}
		}()
		return nil
	}
}

func (m model) fetchKrakendEndpoint() tea.Cmd {
	return func() tea.Msg {
		configMap, err := m.k8sClient.CoreV1().ConfigMaps("default").Get(context.TODO(), "krakend-config", metav1.GetOptions{})
		if err != nil {
			return krakendEndpointMsg{err: fmt.Errorf("Failed to get configmap: %w", err)}
		}

		// dummy parse
		endpoint := "NOT FOUND"
		if data, ok := configMap.Data["krakend.json"]; ok {
			// very basic
			if strings.Contains(data, "my-app-service") {
				endpoint = "/my-app/{id}"
			}
		}

		return krakendEndpointMsg{endpoint: endpoint}
	}
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if err := p.Start(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}


