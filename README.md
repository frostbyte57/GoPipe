
```text
   ______      ____  _
  / ____/___  / __ \(_)___  ___
 / / __/ __ \/ /_/ / / __ \/ _ \
/ /_/ / /_/ / ____/ / /_/ /  __/
\____/\____/_/   /_/ .___/\___/
                  /_/
```

# GoPipe

**GoPipe** is a blazing fast, secure, and simple command-line tool for transferring files between computers. Built with Go for maximum performance, it utilizes high-speed buffered IO (64MB chunks) to ensure your transfers utilize the full potential of your machine and network.

## Features

- **High Performance**: Optimized with 64MB read/write buffers to maximize throughput.
- **Secure**: Uses PAKE (Password Authenticated Key Exchange) for secure connection establishment.
- **Simple TUI**: Beautiful and easy-to-use Terminal User Interface.
- **Cross-Platform**: Works on Windows, macOS, and Linux.

## Installation

### Option 1: Download Binary (Windows Only)
You can simply download the pre-compiled binary for your system, make it executable, and run it.
1. Go to the **[Releases](https://github.com/frostbyte57/GoPipe/releases)** page.
2. Download the version for your OS (e.g., `gopipe.exe` for Windows).
3. Open your terminal and run it!

### Option 2: Install via Go
If you have Go installed on your machine, you can install it directly:

```bash
go install github.com/frostbyte57/GoPipe/cmd/gopipe@latest
```

## 🎮 Usage

Simply run the command in your terminal:

```bash
gopipe
```

### Sending a File
1. Validate that you are in the **Send** tab.
2. Enter the absolute path to the file or directory you want to send.
   - *Directories are automatically zipped!*
3. Share the generated **Wormhole Code** (e.g., `7-231414`) with the receiver.

### Receiving a File
1. Select the **Receive** option.
2. Enter the **Wormhole Code** provided by the sender.
3. The file will be securely transferred and saved to your current directory.

---
*Built with ❤️ in Go.*
