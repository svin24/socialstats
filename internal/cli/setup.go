package cli

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"socialstats/internal/config"
)

// setupInput is the prompt source. Tests replace it with a non-file reader
// so a terminal stdin cannot block `go test`.
var setupInput io.Reader = os.Stdin

func firstTimeSetup(clientSecretPath string) string {
	return fmt.Sprintf(`First-time setup

1. Open Google Cloud Console (https://console.cloud.google.com/projectcreate) and create a project, e.g. "socialstats".
2. Enable both APIs: YouTube Analytics API (https://console.cloud.google.com/apis/library/youtubeanalytics.googleapis.com) and YouTube Data API v3 (https://console.cloud.google.com/apis/library/youtube.googleapis.com).
3. Set up the OAuth consent screen (https://console.cloud.google.com/auth/overview): External, app name + your email, and under Audience add the Google account that owns the channel as a test user.
4. Under Clients (https://console.cloud.google.com/auth/clients), Create client, type Desktop app, download the JSON.
5. Save it as %s and run `+"`socialstats auth`"+`; sign in with the channel's account, allow read-only access, and click Continue on the "unverified app" warning since it's your own app.

Then: run `+"`socialstats fetch`"+`. Also note: in Testing mode Google expires the token after 7 days; publish the app under Audience to avoid weekly re-auth.
`, clientSecretPath)
}

func runSetup(gf globalFlags, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&gf.channelID, "channel-id", gf.channelID, "YouTube channel id; empty auto-detects")
	fs.StringVar(&gf.xHandle, "x-handle", gf.xHandle, "optional X handle label")
	fs.StringVar(&gf.xWatchDir, "watch-dir", gf.xWatchDir, "X CSV watch directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := loadCfg(gf)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := os.MkdirAll(cfg.ConfigDir, 0o700); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := os.Chmod(cfg.ConfigDir, 0o700); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, firstTimeSetup(cfg.ClientSecretPath))
	fmt.Fprintf(stdout, "config dir: %s\n", cfg.ConfigDir)

	channel := strings.TrimSpace(gf.channelID)
	handle := strings.TrimSpace(gf.xHandle)
	if setupIsInteractive() {
		if channel == "" {
			line, err := promptLine(setupInput, stdout, "YouTube channel id [empty = auto-detect]: ")
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			channel = line
		}
		if handle == "" {
			line, err := promptLine(setupInput, stdout, "X handle [optional label]: ")
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			handle = line
		}
	}

	overlay := config.File{
		YouTubeChannelID: channel,
		XHandle:          handle,
		XWatchDir:        strings.TrimSpace(gf.xWatchDir),
	}
	if overlay == (config.File{}) {
		fmt.Fprintln(stdout, "config.json: unchanged (pass --channel-id or --x-handle, or run in a terminal to be prompted)")
		return 0
	}
	if err := config.WriteOverlay(cfg.ConfigFilePath, overlay); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %s\n", cfg.ConfigFilePath)
	return 0
}

func setupIsInteractive() bool {
	f, ok := setupInput.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

func promptLine(r io.Reader, w io.Writer, label string) (string, error) {
	fmt.Fprint(w, label)
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
