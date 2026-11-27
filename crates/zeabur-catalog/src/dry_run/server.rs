//! etcd and kube-apiserver on loopback, with no nodes and no controllers, and kubectl pointed at
//! them. Both binaries, kubectl and openssl come from the dev shell.

use std::fs;
use std::io::Write as _;
use std::net::TcpListener;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::time::{Duration, Instant};

use serde::Deserialize;

/// kubectl with the kubeconfig of one API server.
#[derive(Debug)]
pub(crate) struct Kubectl {
    kubeconfig: PathBuf,
}

/// kubectl failed: how, and what it wrote to stderr.
#[derive(Debug)]
pub(crate) struct Failed {
    /// The command and its exit.
    pub(crate) message: String,
    /// What kubectl wrote to stderr.
    pub(crate) stderr: String,
}

impl Failed {
    /// The failure as one line of text with kubectl's output.
    pub(crate) fn text(&self) -> String {
        format!("{}: {}", self.message, self.stderr.trim())
    }
}

impl Kubectl {
    /// Runs `kubectl <args>` with `input` on stdin and returns its stdout.
    pub(crate) fn run(&self, input: &[u8], args: &[&str]) -> Result<String, Failed> {
        let failed = |message: String| Failed {
            message,
            stderr: String::new(),
        };
        let mut child = Command::new("kubectl")
            .args(args)
            .env("KUBECONFIG", &self.kubeconfig)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
            .map_err(|e| failed(format!("kubectl: {e}")))?;
        let stdin = child.stdin.take();
        // A large input is written while kubectl writes its output, so neither waits on the other.
        let written = std::thread::scope(|s| {
            let writer = s.spawn(move || stdin.map_or(Ok(()), |mut stdin| stdin.write_all(input)));
            let out = child.wait_with_output();
            (writer.join(), out)
        });
        let out = match written {
            (_, Err(e)) => return Err(failed(format!("kubectl: {e}"))),
            (_, Ok(out)) => out,
        };
        let stdout = String::from_utf8_lossy(&out.stdout).into_owned();
        if out.status.success() {
            return Ok(stdout);
        }
        Err(Failed {
            message: format!("kubectl {}: {}", args.join(" "), out.status),
            stderr: String::from_utf8_lossy(&out.stderr).into_owned(),
        })
    }

    /// [`Kubectl::run`], with the failure as text.
    pub(crate) fn text(&self, input: &[u8], args: &[&str]) -> Result<String, String> {
        self.run(input, args).map_err(|f| f.text())
    }
}

/// A running API server. Dropping it stops the processes and removes its directory.
#[derive(Debug)]
pub(crate) struct ApiServer {
    dir: PathBuf,
    procs: Vec<(&'static str, Child)>,
    /// kubectl for this server.
    pub(crate) kubectl: Kubectl,
}

impl ApiServer {
    /// Starts etcd and kube-apiserver with their files in `dir`, which it empties first, and
    /// waits until the API server is ready.
    pub(crate) fn start(dir: &Path) -> Result<Self, String> {
        let dir = std::path::absolute(dir).map_err(|e| format!("{}: {e}", dir.display()))?;
        if dir.exists() {
            fs::remove_dir_all(&dir).map_err(|e| format!("{}: {e}", dir.display()))?;
        }
        fs::create_dir_all(&dir).map_err(|e| format!("{}: {e}", dir.display()))?;
        let mut server = Self {
            kubectl: Kubectl {
                kubeconfig: dir.join("kubeconfig"),
            },
            dir,
            procs: Vec::new(),
        };
        server.run()?;
        Ok(server)
    }

    fn run(&mut self) -> Result<(), String> {
        let [etcd_port, peer_port, api_port] = free_ports()?;
        let etcd = format!("http://127.0.0.1:{etcd_port}");
        let data = self.dir.join("etcd");
        self.spawn(
            "etcd",
            &[
                "--data-dir".as_ref(),
                data.as_os_str(),
                "--listen-client-urls".as_ref(),
                etcd.as_ref(),
                "--advertise-client-urls".as_ref(),
                etcd.as_ref(),
                "--listen-peer-urls".as_ref(),
                format!("http://127.0.0.1:{peer_port}").as_ref(),
                "--unsafe-no-fsync".as_ref(),
            ],
        )?;
        let key = self.dir.join("sa.key");
        let out = Command::new("openssl")
            .args([
                "ecparam",
                "-name",
                "prime256v1",
                "-genkey",
                "-noout",
                "-out",
            ])
            .arg(&key)
            .output()
            .map_err(|e| format!("openssl: {e}"))?;
        if !out.status.success() {
            return Err(format!(
                "openssl: {}",
                String::from_utf8_lossy(&out.stderr).trim()
            ));
        }
        let mut token = [0; 24];
        crate::secrets::os_random(&mut token)?;
        let token = crate::render::hex(&token);
        let tokens = self.dir.join("tokens.csv");
        fs::write(&tokens, format!("{token},admin,admin,system:masters\n"))
            .map_err(|e| format!("{}: {e}", tokens.display()))?;
        let certs = self.dir.join("certs");
        // This server has no node and no second API server, so it does not reconcile the
        // endpoints of the kubernetes Service. The lease reconciler refuses a loopback address
        // since 1.37; cmd/kube-apiserver/app/testing/testserver.go disables it in the same way.
        self.spawn(
            "kube-apiserver",
            &[
                "--etcd-servers".as_ref(),
                etcd.as_ref(),
                "--bind-address=127.0.0.1".as_ref(),
                "--advertise-address=127.0.0.1".as_ref(),
                "--endpoint-reconciler-type=none".as_ref(),
                format!("--secure-port={api_port}").as_ref(),
                "--cert-dir".as_ref(),
                certs.as_os_str(),
                "--token-auth-file".as_ref(),
                tokens.as_os_str(),
                "--authorization-mode=RBAC".as_ref(),
                "--service-account-issuer=https://kubernetes.default.svc".as_ref(),
                "--service-account-key-file".as_ref(),
                key.as_os_str(),
                "--service-account-signing-key-file".as_ref(),
                key.as_os_str(),
                "--service-cluster-ip-range=10.96.0.0/16".as_ref(),
                "--allow-privileged".as_ref(),
            ],
        )?;
        let config = format!(
            "apiVersion: v1\nkind: Config\nclusters: [{{name: check, cluster: {{server: \
             \"https://127.0.0.1:{api_port}\", certificate-authority: {:?}}}}}]\nusers: [{{name: \
             admin, user: {{token: {token:?}}}}}]\ncontexts: [{{name: check, context: {{cluster: \
             check, user: admin}}}}]\ncurrent-context: check\n",
            certs.join("apiserver.crt").display().to_string(),
        );
        fs::write(&self.kubectl.kubeconfig, config)
            .map_err(|e| format!("{}: {e}", self.kubectl.kubeconfig.display()))?;
        let deadline = Instant::now() + Duration::from_secs(120);
        loop {
            let ready = self.kubectl.run(&[], &["get", "--raw", "/readyz"]);
            let Err(not_ready) = ready else {
                return Ok(());
            };
            for (name, child) in &mut self.procs {
                if child.try_wait().is_ok_and(|s| s.is_some()) {
                    let log = fs::read_to_string(self.dir.join(format!("{name}.log")))
                        .unwrap_or_default();
                    return Err(format!("{name} stopped: {}", tail(&log, 20)));
                }
            }
            if Instant::now() > deadline {
                return Err(format!(
                    "the API server is not ready after 2m: {}",
                    not_ready.text()
                ));
            }
            std::thread::sleep(Duration::from_secs(1));
        }
    }

    /// Starts `name` with its output in `<dir>/<name>.log`.
    fn spawn(&mut self, name: &'static str, args: &[&std::ffi::OsStr]) -> Result<(), String> {
        let path = self.dir.join(format!("{name}.log"));
        let log = fs::File::create(&path).map_err(|e| format!("{}: {e}", path.display()))?;
        let err = log
            .try_clone()
            .map_err(|e| format!("{}: {e}", path.display()))?;
        crate::output::info(&format!("starting {name}"));
        let child = Command::new(name)
            .args(args)
            .stdin(Stdio::null())
            .stdout(log)
            .stderr(err)
            .spawn()
            .map_err(|e| format!("{name}: {e}"))?;
        self.procs.push((name, child));
        Ok(())
    }

    /// The server's Kubernetes version, such as `1.37.2`.
    pub(crate) fn version(&self) -> Result<String, String> {
        #[derive(Deserialize)]
        #[serde(rename_all = "camelCase")]
        struct Versions {
            server_version: Info,
        }
        #[derive(Deserialize)]
        #[serde(rename_all = "camelCase")]
        struct Info {
            git_version: String,
        }
        let out = self.kubectl.text(&[], &["version", "-o", "json"])?;
        let versions: Versions =
            serde_json::from_str(&out).map_err(|e| format!("kubectl version: {e}"))?;
        let version = versions.server_version.git_version;
        Ok(version.strip_prefix('v').unwrap_or(&version).to_owned())
    }
}

impl Drop for ApiServer {
    fn drop(&mut self) {
        for (_, child) in self.procs.iter_mut().rev() {
            // The data is thrown away, so the processes need no clean shutdown.
            if child.kill().is_ok() {
                let _status: std::io::Result<_> = child.wait();
            }
        }
        let _removed: std::io::Result<()> = fs::remove_dir_all(&self.dir);
    }
}

/// The last `n` lines of `text`.
pub(crate) fn tail(text: &str, n: usize) -> String {
    let lines: Vec<&str> = text.trim().lines().collect();
    lines
        .get(lines.len().saturating_sub(n)..)
        .unwrap_or_default()
        .join("\n")
}

/// Three free ports on loopback.
fn free_ports() -> Result<[u16; 3], String> {
    let listen = || TcpListener::bind("127.0.0.1:0").map_err(|e| format!("a free port: {e}"));
    // All three stay bound until each has its port, so they differ.
    let listeners = [listen()?, listen()?, listen()?];
    let port = |l: &TcpListener| {
        l.local_addr()
            .map(|a| a.port())
            .map_err(|e| format!("a free port: {e}"))
    };
    Ok([
        port(&listeners[0])?,
        port(&listeners[1])?,
        port(&listeners[2])?,
    ])
}
