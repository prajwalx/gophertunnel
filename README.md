# GopherTunnel 🐹🚀

**GopherTunnel** is a high-performance, zero-config P2P file transfer CLI tool built in Go. It enables instant, encrypted file sharing between machines on a local network without the need for cloud intermediaries, third-party servers, or manual IP entry.

---

## ✨ Key Features

* **Zero-Config Discovery:** Uses mDNS (Multicast DNS) with a multi-burst retry pattern to find peers automatically.
* **O(1) Memory Footprint:** Streams data using `io.Copy` and `bufio` wrappers, maintaining <20MB RAM usage regardless of file size (tested with multi-GB files).
* **End-to-End Encryption:** Implements **AES-256-CTR** for byte-stream encryption, ensuring data is never sent in plain text.
* **Cross-Platform Resilience:** Native support for Windows, macOS, and Linux, with specific fixes for cross-OS TCP behavior and network buffer flushing.
* **Graceful Termination:** Full integration with Go `context` and `os/signal` to ensure clean socket and file closure on `Ctrl+C`.

---

## 🏗️ Technical Architecture

### 1. The TCP Close Race Condition (Mac ↔ Win)
During development, a race condition was identified where the Sender (macOS) would close the TCP socket before the Receiver (Windows) could flush the final bytes from the network buffer to the disk. 

**The Solution:** Implemented a **Synchronous Handshake**. The Sender performs a buffer flush and waits for a 1-byte ACK from the Receiver's disk-write routine before terminating the process.



### 2. Multi-Burst mDNS Discovery
mDNS relies on UDP Multicast, which is often lossy on congested WiFi networks.
**The Solution:** Developed a multi-burst discovery pattern that performs parallel queries with internal retries and **TXT-record validation** to ensure the client connects to a verified GopherTunnel peer rather than a random network device (like a router or printer).

### 3. Encrypted Streaming with `io.MultiWriter`
To maintain a real-time progress bar while encrypting data on-the-fly:
* Wrapped the `net.Conn` in a `bufio.Writer` to optimize throughput.
* Used `io.MultiWriter` to split the stream simultaneously to the network socket and the terminal UI.
* Piped the data through `cipher.StreamWriter` for transparent, high-speed AES-CTR encryption.



---

## 🚀 Installation & Usage

### Build from Source
```bash
# Clone the repository
git clone https://github.com/prajwalx/gophertunnel
cd gophertunnel

# Install dependencies
go mod tidy

# Build the binary
go build -o tunnel ./cmd/tunnel
```

### Sending a file

```bash
./tunnel send path/to/your-file.zip
```

### Receiving a file

```bash
./tunnel receive
```

---

## 📂 Project Structure
```bash
├── cmd/
│   └── tunnel/           # CLI Entry point & Cobra Commands
├── internal/
│   ├── discovery/        # mDNS Server/Client & Peer Validation
│   ├── security/         # AES-CTR Encryption logic
│   └── transfer/         # TCP Protocol, Handshaking, and Streaming
└── go.mod                # Dependency management
```
---
## 🛡️ Security Note

This tool uses a hardcoded shared key for demonstration purposes. In a production environment, this should be replaced with a key derived from a user-provided passphrase using a KDF like Argon2, or an asymmetric exchange (Diffie-Hellman).

---
## 🤝 Contributing
Feel free to open issues or submit PRs. Current roadmap includes:

[ ] Directory/Folder recursive transfer.

[ ] UPnP support for Wide Area Network (WAN) transfers.

[ ] GUI wrapper using Fyne or Wails.