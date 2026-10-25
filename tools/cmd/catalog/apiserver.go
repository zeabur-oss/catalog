package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

// apiServer is etcd and kube-apiserver on loopback, with no nodes and no
// controllers. It has its own kubeconfig. Both binaries come from the dev shell.
type apiServer struct {
	dir   string
	procs []proc
}

type proc struct {
	cmd  *exec.Cmd
	done chan struct{}
}

func (p proc) exited() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

// startAPIServer starts the API server and points KUBECONFIG at it.
func startAPIServer() (*apiServer, error) {
	dir, err := os.MkdirTemp("", "catalog-apiserver-")
	if err != nil {
		return nil, err
	}
	s := &apiServer{dir: dir}
	if err := s.start(); err != nil {
		s.stop()
		return nil, err
	}
	return s, nil
}

func (s *apiServer) start() error {
	ports, err := freePorts(3)
	if err != nil {
		return err
	}
	etcd := fmt.Sprintf("http://127.0.0.1:%d", ports[0])
	if err := s.run("etcd", "--data-dir", filepath.Join(s.dir, "etcd"),
		"--listen-client-urls", etcd, "--advertise-client-urls", etcd,
		"--listen-peer-urls", fmt.Sprintf("http://127.0.0.1:%d", ports[1]),
		"--unsafe-no-fsync"); err != nil {
		return err
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	keyFile := filepath.Join(s.dir, "sa.key")
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		return err
	}
	token := rand.Text()
	if err := os.WriteFile(filepath.Join(s.dir, "tokens.csv"), []byte(token+",admin,admin,system:masters\n"), 0o600); err != nil {
		return err
	}
	certs := filepath.Join(s.dir, "certs")
	if err := s.run("kube-apiserver", "--etcd-servers", etcd,
		"--bind-address", "127.0.0.1", "--advertise-address", "127.0.0.1",
		"--secure-port", fmt.Sprint(ports[2]), "--cert-dir", certs,
		"--token-auth-file", filepath.Join(s.dir, "tokens.csv"), "--authorization-mode", "RBAC",
		"--service-account-issuer", "https://kubernetes.default.svc",
		"--service-account-key-file", keyFile, "--service-account-signing-key-file", keyFile,
		"--service-cluster-ip-range", "10.96.0.0/16", "--allow-privileged"); err != nil {
		return err
	}
	kubeconfig := filepath.Join(s.dir, "kubeconfig")
	cfg := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters: [{name: l0, cluster: {server: "https://127.0.0.1:%d", certificate-authority: %q}}]
users: [{name: admin, user: {token: %q}}]
contexts: [{name: l0, context: {cluster: l0, user: admin}}]
current-context: l0
`, ports[2], filepath.Join(certs, "apiserver.crt"), token)
	if err := os.WriteFile(kubeconfig, []byte(cfg), 0o600); err != nil {
		return err
	}
	os.Setenv("KUBECONFIG", kubeconfig)
	deadline := time.Now().Add(2 * time.Minute)
	for {
		_, err := catalog.Run(nil, "kubectl", "get", "--raw", "/readyz")
		if err == nil {
			return nil
		}
		for _, p := range s.procs {
			if p.exited() {
				log, _ := os.ReadFile(filepath.Join(s.dir, filepath.Base(p.cmd.Path)+".log"))
				return fmt.Errorf("%s stopped: %s", filepath.Base(p.cmd.Path), tail(string(log), 20))
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("API server not ready after 2m: %w", err)
		}
		time.Sleep(time.Second)
	}
}

// run starts name in the background with its output in the server's directory.
func (s *apiServer) run(name string, args ...string) error {
	log, err := os.Create(filepath.Join(s.dir, name+".log"))
	if err != nil {
		return err
	}
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = log, log
	slog.Info("starting", "cmd", name)
	if err := cmd.Start(); err != nil {
		log.Close()
		return err
	}
	p := proc{cmd, make(chan struct{})}
	s.procs = append(s.procs, p)
	go func() { cmd.Wait(); log.Close(); close(p.done) }()
	return nil
}

// version returns the server's Kubernetes version, such as 1.36.5.
func (s *apiServer) version() (string, error) {
	out, err := catalog.Run(nil, "kubectl", "version", "-o", "json")
	if err != nil {
		return "", err
	}
	var v struct {
		ServerVersion struct{ GitVersion string } `json:"serverVersion"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return "", err
	}
	return strings.TrimPrefix(v.ServerVersion.GitVersion, "v"), nil
}

func (s *apiServer) stop() {
	for i := len(s.procs) - 1; i >= 0; i-- {
		p := s.procs[i]
		p.cmd.Process.Signal(os.Interrupt)
		select {
		case <-p.done:
		case <-time.After(10 * time.Second):
			p.cmd.Process.Kill()
			<-p.done
		}
	}
	os.RemoveAll(s.dir)
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.Join(lines[max(0, len(lines)-n):], "\n")
}

func freePorts(n int) ([]int, error) {
	var ports []int
	for range n {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		defer l.Close()
		ports = append(ports, l.Addr().(*net.TCPAddr).Port)
	}
	return ports, nil
}
