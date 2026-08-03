package collectors

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// SwarmInfo is a compact view of the local engine's swarm membership.
type SwarmInfo struct {
	Active        bool
	Local         bool
	State         string // active | inactive | pending | locked | error
	NodeID        string
	ClusterID     string
	Managers      int
	Nodes         int
	Workers       int
	Services      int
	Tasks         int
	Networks      int
	EngineVersion string
	Error         string
}

type SwarmNode struct {
	ID           string
	Hostname     string
	Status       string
	Availability string
	Manager      string
	Engine       string
	Addr         string
	Role         string // manager | worker
}

type SwarmService struct {
	ID       string
	Name     string
	Mode     string
	Replicas string
	Image    string
	Ports    string
}

type SwarmTask struct {
	ID           string
	Name         string
	Service      string
	Node         string
	DesiredState string
	CurrentState string
	Error        string
	Image        string
}

type SwarmStack struct {
	Name     string
	Services int
	Orchestr string
}

type SwarmNetwork struct {
	ID     string
	Name   string
	Driver string
	Scope  string
}

type SwarmSecret struct {
	ID        string
	Name      string
	CreatedAt string
	UpdatedAt string
}

type SwarmConfig struct {
	ID        string
	Name      string
	CreatedAt string
	UpdatedAt string
}

type SwarmEvent struct {
	Time     string
	Type     string
	Action   string
	Resource string
}

// SwarmAvailable reports whether the docker CLI is on PATH.
func SwarmAvailable() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

func runDocker(timeout time.Duration, args ...string) (string, error) {
	cmd := exec.Command("docker", args...)
	done := make(chan struct{})
	var out []byte
	var err error
	go func() {
		out, err = cmd.CombinedOutput()
		close(done)
	}()
	select {
	case <-done:
		s := strings.TrimSpace(string(out))
		if err != nil {
			if s != "" {
				return s, fmt.Errorf("%s", firstLine(s))
			}
			return s, err
		}
		return s, nil
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return "", fmt.Errorf("docker timeout")
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

// SwarmClusterInfo reads swarm state from `docker info`.
func SwarmClusterInfo() SwarmInfo {
	info := SwarmInfo{}
	if !SwarmAvailable() {
		info.Error = "docker não encontrado"
		info.State = "unavailable"
		return info
	}
	out, err := runDocker(10*time.Second, "info", "--format",
		"{{.Swarm.LocalNodeState}}\t{{.Swarm.NodeID}}\t{{.Swarm.Managers}}\t{{.Swarm.Nodes}}\t{{.Swarm.Cluster.ID}}\t{{.ServerVersion}}")
	if err != nil {
		info.Error = err.Error()
		info.State = "unavailable"
		return info
	}
	parts := strings.Split(out, "\t")
	state := ""
	if len(parts) > 0 {
		state = strings.TrimSpace(parts[0])
	}
	info.State = strings.ToLower(state)
	if info.State == "" {
		info.State = "inactive"
	}
	info.Active = strings.EqualFold(state, "active")
	if len(parts) > 1 {
		info.NodeID = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 {
		info.Managers, _ = strconv.Atoi(strings.TrimSpace(parts[2]))
	}
	if len(parts) > 3 {
		info.Nodes, _ = strconv.Atoi(strings.TrimSpace(parts[3]))
	}
	if len(parts) > 4 {
		info.ClusterID = strings.TrimSpace(parts[4])
	}
	if len(parts) > 5 {
		info.EngineVersion = strings.TrimSpace(parts[5])
	}
	if info.Nodes >= info.Managers {
		info.Workers = info.Nodes - info.Managers
	}
	info.Local = info.Active && info.Nodes <= 1
	return info
}

func SwarmListNodes() ([]SwarmNode, error) {
	out, err := runDocker(12*time.Second, "node", "ls", "--format",
		"{{.ID}}\t{{.Hostname}}\t{{.Status}}\t{{.Availability}}\t{{.ManagerStatus}}\t{{.EngineVersion}}")
	if err != nil {
		return nil, err
	}
	var nodes []SwarmNode
	for _, line := range splitNonEmpty(out) {
		p := strings.Split(line, "\t")
		if len(p) < 4 {
			continue
		}
		n := SwarmNode{
			ID:           p[0],
			Hostname:     p[1],
			Status:       p[2],
			Availability: p[3],
			Role:         "worker",
		}
		if len(p) > 4 {
			n.Manager = p[4]
			if n.Manager != "" {
				n.Role = "manager"
			}
		}
		if len(p) > 5 {
			n.Engine = p[5]
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func SwarmListServices() ([]SwarmService, error) {
	out, err := runDocker(12*time.Second, "service", "ls", "--format",
		"{{.ID}}\t{{.Name}}\t{{.Mode}}\t{{.Replicas}}\t{{.Image}}\t{{.Ports}}")
	if err != nil {
		return nil, err
	}
	var services []SwarmService
	for _, line := range splitNonEmpty(out) {
		p := strings.Split(line, "\t")
		if len(p) < 4 {
			continue
		}
		s := SwarmService{
			ID:       p[0],
			Name:     p[1],
			Mode:     p[2],
			Replicas: p[3],
		}
		if len(p) > 4 {
			s.Image = p[4]
		}
		if len(p) > 5 {
			s.Ports = p[5]
		}
		services = append(services, s)
	}
	return services, nil
}

func SwarmListTasks() ([]SwarmTask, error) {
	// ponytail: aggregate via services; docker node ps needs shell for multi-id
	svcs, err := SwarmListServices()
	if err != nil {
		return nil, err
	}
	var tasks []SwarmTask
	for _, s := range svcs {
		list, err2 := SwarmListServiceTasks(s.Name)
		if err2 != nil {
			continue
		}
		tasks = append(tasks, list...)
	}
	return tasks, nil
}

func SwarmListServiceTasks(service string) ([]SwarmTask, error) {
	out, err := runDocker(12*time.Second, "service", "ps", service, "--no-trunc", "--format",
		"{{.ID}}\t{{.Name}}\t{{.Node}}\t{{.DesiredState}}\t{{.CurrentState}}\t{{.Error}}\t{{.Image}}")
	if err != nil {
		return nil, err
	}
	return parseSwarmTasks(out, service), nil
}

func parseSwarmTasks(out, defaultService string) []SwarmTask {
	var tasks []SwarmTask
	for _, line := range splitNonEmpty(out) {
		p := strings.Split(line, "\t")
		if len(p) < 5 {
			continue
		}
		name := p[1]
		svc := defaultService
		if svc == "" {
			if i := strings.LastIndex(name, "."); i > 0 {
				svc = name[:i]
				// strip replica index: api.1 → api; stack_api.1 → stack_api
				if j := strings.LastIndex(svc, "."); j > 0 {
					if _, err := strconv.Atoi(svc[j+1:]); err == nil {
						svc = svc[:j]
					}
				}
			}
		}
		t := SwarmTask{
			ID:           p[0],
			Name:         name,
			Service:      svc,
			Node:         p[2],
			DesiredState: p[3],
			CurrentState: p[4],
		}
		if len(p) > 5 {
			t.Error = p[5]
		}
		if len(p) > 6 {
			t.Image = p[6]
		}
		tasks = append(tasks, t)
	}
	return tasks
}

func SwarmListStacks() ([]SwarmStack, error) {
	out, err := runDocker(12*time.Second, "stack", "ls", "--format",
		"{{.Name}}\t{{.Services}}\t{{.Orchestrator}}")
	if err != nil {
		return nil, err
	}
	var stacks []SwarmStack
	for _, line := range splitNonEmpty(out) {
		p := strings.Split(line, "\t")
		if len(p) < 1 || p[0] == "" {
			continue
		}
		s := SwarmStack{Name: p[0], Orchestr: "swarm"}
		if len(p) > 1 {
			s.Services, _ = strconv.Atoi(strings.TrimSpace(p[1]))
		}
		if len(p) > 2 && p[2] != "" {
			s.Orchestr = p[2]
		}
		stacks = append(stacks, s)
	}
	return stacks, nil
}

func SwarmListNetworks() ([]SwarmNetwork, error) {
	out, err := runDocker(12*time.Second, "network", "ls", "--filter", "scope=swarm", "--format",
		"{{.ID}}\t{{.Name}}\t{{.Driver}}\t{{.Scope}}")
	if err != nil {
		return nil, err
	}
	var nets []SwarmNetwork
	for _, line := range splitNonEmpty(out) {
		p := strings.Split(line, "\t")
		if len(p) < 2 {
			continue
		}
		n := SwarmNetwork{ID: p[0], Name: p[1]}
		if len(p) > 2 {
			n.Driver = p[2]
		}
		if len(p) > 3 {
			n.Scope = p[3]
		}
		nets = append(nets, n)
	}
	return nets, nil
}

func SwarmListSecrets() ([]SwarmSecret, error) {
	out, err := runDocker(12*time.Second, "secret", "ls", "--format",
		"{{.ID}}\t{{.Name}}\t{{.CreatedAt}}\t{{.UpdatedAt}}")
	if err != nil {
		return nil, err
	}
	var list []SwarmSecret
	for _, line := range splitNonEmpty(out) {
		p := strings.Split(line, "\t")
		if len(p) < 2 {
			continue
		}
		s := SwarmSecret{ID: p[0], Name: p[1]}
		if len(p) > 2 {
			s.CreatedAt = p[2]
		}
		if len(p) > 3 {
			s.UpdatedAt = p[3]
		}
		list = append(list, s)
	}
	return list, nil
}

func SwarmListConfigs() ([]SwarmConfig, error) {
	out, err := runDocker(12*time.Second, "config", "ls", "--format",
		"{{.ID}}\t{{.Name}}\t{{.CreatedAt}}\t{{.UpdatedAt}}")
	if err != nil {
		return nil, err
	}
	var list []SwarmConfig
	for _, line := range splitNonEmpty(out) {
		p := strings.Split(line, "\t")
		if len(p) < 2 {
			continue
		}
		c := SwarmConfig{ID: p[0], Name: p[1]}
		if len(p) > 2 {
			c.CreatedAt = p[2]
		}
		if len(p) > 3 {
			c.UpdatedAt = p[3]
		}
		list = append(list, c)
	}
	return list, nil
}

// SwarmRecentEvents snapshots recent docker events (non-streaming).
func SwarmRecentEvents(since string) ([]SwarmEvent, error) {
	if since == "" {
		since = "15m"
	}
	until := time.Now().UTC().Format(time.RFC3339)
	out, err := runDocker(12*time.Second, "events",
		"--since", since,
		"--until", until,
		"--filter", "type=service",
		"--filter", "type=node",
		"--filter", "type=container",
		"--format", "{{.Time}}\t{{.Type}}\t{{.Action}}\t{{index .Actor.Attributes \"name\"}}")
	if err != nil && out == "" {
		return nil, err
	}
	var events []SwarmEvent
	lines := splitNonEmpty(out)
	// newest last from docker; reverse for newest-first UI
	for i := len(lines) - 1; i >= 0; i-- {
		p := strings.Split(lines[i], "\t")
		if len(p) < 3 {
			continue
		}
		e := SwarmEvent{Time: p[0], Type: p[1], Action: p[2]}
		if len(p) > 3 {
			e.Resource = p[3]
		}
		events = append(events, e)
		if len(events) >= 40 {
			break
		}
	}
	return events, nil
}

func SwarmInspectService(name string) (string, error) {
	return runDocker(12*time.Second, "service", "inspect", "--pretty", name)
}

func SwarmInspectNode(idOrName string) (string, error) {
	return runDocker(12*time.Second, "node", "inspect", "--pretty", idOrName)
}

func SwarmInspectNetwork(name string) (string, error) {
	return runDocker(12*time.Second, "network", "inspect", name)
}

func SwarmInspectSecret(name string) (string, error) {
	// Never dump secret data — format only metadata fields.
	return runDocker(12*time.Second, "secret", "inspect", "--format",
		"Name: {{.Spec.Name}}\nID: {{.ID}}\nCreated: {{.CreatedAt}}\nUpdated: {{.UpdatedAt}}", name)
}

func SwarmInspectConfig(name string) (string, error) {
	return runDocker(12*time.Second, "config", "inspect", "--format",
		"Name: {{.Spec.Name}}\nID: {{.ID}}\nCreated: {{.CreatedAt}}\nUpdated: {{.UpdatedAt}}\n---\n{{printf \"%s\" .Spec.Data}}", name)
}

func SwarmServiceTasks(name string) (string, error) {
	return runDocker(12*time.Second, "service", "ps", name, "--no-trunc")
}

func SwarmServiceLogs(name string, tail int) (string, error) {
	if tail <= 0 {
		tail = 80
	}
	return runDocker(15*time.Second, "service", "logs", "--tail", strconv.Itoa(tail), name)
}

func SwarmServiceScale(name string, replicas int) error {
	if replicas < 0 {
		replicas = 0
	}
	_, err := runDocker(30*time.Second, "service", "scale", fmt.Sprintf("%s=%d", name, replicas))
	return err
}

func SwarmServiceRemove(name string) error {
	_, err := runDocker(30*time.Second, "service", "rm", name)
	return err
}

func SwarmServiceForceUpdate(name string) (string, error) {
	return runDocker(45*time.Second, "service", "update", "--force", name)
}

func SwarmServiceUpdateImage(name, image string) (string, error) {
	return runDocker(60*time.Second, "service", "update", "--image", image, name)
}

func SwarmServiceRollback(name string) (string, error) {
	return runDocker(45*time.Second, "service", "rollback", name)
}

func SwarmServiceCreate(name, image string, replicas int, publish string, network string) (string, error) {
	args := []string{"service", "create", "--name", name, "--replicas", strconv.Itoa(replicas)}
	if publish != "" {
		args = append(args, "--publish", publish)
	}
	if network != "" {
		args = append(args, "--network", network)
	}
	args = append(args, image)
	return runDocker(60*time.Second, args...)
}

func SwarmInit(advertiseAddr string) (string, error) {
	args := []string{"swarm", "init"}
	if advertiseAddr != "" {
		args = append(args, "--advertise-addr", advertiseAddr)
	}
	return runDocker(20*time.Second, args...)
}

func SwarmLeave(force bool) (string, error) {
	args := []string{"swarm", "leave"}
	if force {
		args = append(args, "--force")
	}
	return runDocker(20*time.Second, args...)
}

func SwarmJoinToken(manager bool) (string, error) {
	role := "worker"
	if manager {
		role = "manager"
	}
	return runDocker(10*time.Second, "swarm", "join-token", role)
}

func SwarmNodePromote(idOrName string) (string, error) {
	return runDocker(20*time.Second, "node", "promote", idOrName)
}

func SwarmNodeDemote(idOrName string) (string, error) {
	return runDocker(20*time.Second, "node", "demote", idOrName)
}

func SwarmNodeAvailability(idOrName, avail string) (string, error) {
	avail = strings.ToLower(strings.TrimSpace(avail))
	switch avail {
	case "active", "pause", "drain":
	default:
		return "", fmt.Errorf("availability inválida: %s", avail)
	}
	return runDocker(20*time.Second, "node", "update", "--availability", avail, idOrName)
}

func SwarmNodeRemove(idOrName string, force bool) error {
	args := []string{"node", "rm"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, idOrName)
	_, err := runDocker(20*time.Second, args...)
	return err
}

func SwarmStackDeploy(composePath, name string) (string, error) {
	return runDocker(60*time.Second, "stack", "deploy", "-c", composePath, name)
}

func SwarmStackRemove(name string) error {
	_, err := runDocker(30*time.Second, "stack", "rm", name)
	return err
}

func SwarmStackServices(name string) (string, error) {
	return runDocker(12*time.Second, "stack", "services", name)
}

func SwarmPruneNetworks() (string, error) {
	return runDocker(30*time.Second, "network", "prune", "-f")
}

func SwarmSecretRemove(name string) (string, error) {
	return runDocker(20*time.Second, "secret", "rm", name)
}

func SwarmConfigRemove(name string) (string, error) {
	return runDocker(20*time.Second, "config", "rm", name)
}

// DiscoverSwarmCompose finds a compose file suitable for stack deploy.
func DiscoverSwarmCompose(projectPath string) string {
	candidates := []string{
		"docker-compose.swarm.yml",
		"docker-compose.swarm.yaml",
		"compose.swarm.yml",
		"compose.swarm.yaml",
		"docker-stack.yml",
		"docker-stack.yaml",
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}
	for _, name := range candidates {
		p := filepath.Join(projectPath, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// ParseServiceReplicas extracts the desired replica count from "2/2" or "0/3".
func ParseServiceReplicas(replicas string) (running, desired int) {
	replicas = strings.TrimSpace(replicas)
	if i := strings.IndexByte(replicas, '/'); i >= 0 {
		running, _ = strconv.Atoi(replicas[:i])
		desired, _ = strconv.Atoi(replicas[i+1:])
		return
	}
	desired, _ = strconv.Atoi(replicas)
	return desired, desired
}

// SwarmServiceStatus derives a simple status label from replica counts / mode.
func SwarmServiceStatus(replicas string) string {
	r, d := ParseServiceReplicas(replicas)
	if d == 0 && r == 0 {
		return "shutdown"
	}
	if r == 0 && d > 0 {
		return "pending"
	}
	if r < d {
		return "updating"
	}
	return "running"
}

// SwarmBelongsToProject hints whether a resource name is tied to the project.
func SwarmBelongsToProject(resourceName, projectName string) bool {
	if projectName == "" || resourceName == "" {
		return false
	}
	p := strings.ToLower(sanitizeStackToken(projectName))
	n := strings.ToLower(resourceName)
	return strings.HasPrefix(n, p) || strings.Contains(n, p)
}

func sanitizeStackToken(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
