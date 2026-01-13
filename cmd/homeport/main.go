package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var apiURL = "http://localhost:8080/api"

func main() {
	rootCmd := &cobra.Command{
		Use:   "homeport",
		Short: "Homeport CLI - manage your dev servers",
	}

	// list command
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all detected ports",
		Run:   runList,
	}

	// share command
	shareCmd := &cobra.Command{
		Use:   "share <port>",
		Short: "Share a port (default: private)",
		Args:  cobra.ExactArgs(1),
		Run:   runShare,
	}
	shareCmd.Flags().Bool("public", false, "Make port publicly accessible")
	shareCmd.Flags().Bool("password", false, "Require password for access")
	shareCmd.Flags().StringP("pass", "p", "", "Password (prompts if not provided)")

	// unshare command
	unshareCmd := &cobra.Command{
		Use:   "unshare <port>",
		Short: "Remove sharing from a port",
		Args:  cobra.ExactArgs(1),
		Run:   runUnshare,
	}

	// url command
	urlCmd := &cobra.Command{
		Use:   "url <port>",
		Short: "Get the shareable URL for a port",
		Args:  cobra.ExactArgs(1),
		Run:   runURL,
	}

	// status command
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		Run:   runStatus,
	}

	// repos command
	reposCmd := &cobra.Command{
		Use:   "repos",
		Short: "List cloned repositories",
		Run:   runRepos,
	}

	rootCmd.AddCommand(listCmd, shareCmd, unshareCmd, urlCmd, statusCmd, reposCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

type Port struct {
	Port        int    `json:"port"`
	RepoID      string `json:"repo_id"`
	RepoName    string `json:"repo_name"`
	PID         int    `json:"pid"`
	ProcessName string `json:"process_name"`
	ShareMode   string `json:"share_mode"`
}

type Status struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Uptime  string `json:"uptime"`
	Config  struct {
		PortRange   string `json:"port_range"`
		ExternalURL string `json:"external_url"`
		DevMode     bool   `json:"dev_mode"`
	} `json:"config"`
}

type Repo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	GitHubURL string `json:"github_url"`
}

func runList(cmd *cobra.Command, args []string) {
	resp, err := http.Get(apiURL + "/ports")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var ports []Port
	if err := json.NewDecoder(resp.Body).Decode(&ports); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(ports) == 0 {
		fmt.Println("No ports detected")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PORT\tPROCESS\tREPO\tSHARE MODE")
	for _, p := range ports {
		repo := p.RepoName
		if repo == "" {
			repo = "-"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", p.Port, p.ProcessName, repo, p.ShareMode)
	}
	w.Flush()
}

func runShare(cmd *cobra.Command, args []string) {
	port := args[0]

	isPublic, _ := cmd.Flags().GetBool("public")
	isPassword, _ := cmd.Flags().GetBool("password")
	password, _ := cmd.Flags().GetString("pass")

	mode := "private"
	if isPublic {
		mode = "public"
	} else if isPassword {
		mode = "password"
		if password == "" {
			fmt.Print("Enter password: ")
			fmt.Scanln(&password)
		}
	}

	body := fmt.Sprintf(`{"mode":"%s"`, mode)
	if mode == "password" {
		body += fmt.Sprintf(`,"password":"%s"`, password)
	}
	body += "}"

	req, _ := http.NewRequest("POST", apiURL+"/share/"+port, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "Error: %s\n", body)
		os.Exit(1)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	fmt.Printf("Port %s shared as %s\n", port, mode)
	fmt.Printf("URL: %s\n", result["url"])
}

func runUnshare(cmd *cobra.Command, args []string) {
	port := args[0]

	req, _ := http.NewRequest("DELETE", apiURL+"/share/"+port, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "Error: %s\n", body)
		os.Exit(1)
	}

	fmt.Printf("Port %s unshared (now private)\n", port)
}

func runURL(cmd *cobra.Command, args []string) {
	port := args[0]

	// Get status to get external URL
	resp, err := http.Get(apiURL + "/status")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var status Status
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s/%s/\n", status.Config.ExternalURL, port)
}

func runStatus(cmd *cobra.Command, args []string) {
	resp, err := http.Get(apiURL + "/status")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot connect to daemon at %s\n", apiURL)
		fmt.Fprintf(os.Stderr, "Is homeportd running?\n")
		os.Exit(1)
	}
	defer resp.Body.Close()

	var status Status
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Homeport v%s\n", status.Version)
	fmt.Printf("Status: %s\n", status.Status)
	fmt.Printf("Uptime: %s\n", status.Uptime)
	fmt.Printf("Port range: %s\n", status.Config.PortRange)
	fmt.Printf("External URL: %s\n", status.Config.ExternalURL)
	if status.Config.DevMode {
		fmt.Println("Mode: development")
	} else {
		fmt.Println("Mode: production")
	}

	// Get port count
	portsResp, err := http.Get(apiURL + "/ports")
	if err == nil {
		defer portsResp.Body.Close()
		var ports []Port
		if json.NewDecoder(portsResp.Body).Decode(&ports) == nil {
			fmt.Printf("Active ports: %d\n", len(ports))
		}
	}
}

func runRepos(cmd *cobra.Command, args []string) {
	resp, err := http.Get(apiURL + "/repos")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var repos []Repo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(repos) == 0 {
		fmt.Println("No repositories cloned")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tPATH")
	for _, r := range repos {
		fmt.Fprintf(w, "%s\t%s\n", r.Name, r.Path)
	}
	w.Flush()
}

// Unused but keeping for reference
func parseInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
