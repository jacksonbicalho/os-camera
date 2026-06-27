package deviceinfo

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWebcamDetect(t *testing.T) {
	w := NewWebcam()
	if !w.Detect(context.Background(), Target{RTSPURL: "rtsp://127.0.0.1:8554/webcam0"}) {
		t.Error("deveria detectar o restream local")
	}
	for _, url := range []string{
		"rtsp://192.168.1.10:554/stream",
		"rtsp://127.0.0.1:8554/outro",
		"rtsp://127.0.0.1:554/webcam0",
		"",
	} {
		if w.Detect(context.Background(), Target{RTSPURL: url}) {
			t.Errorf("não deveria detectar %q", url)
		}
	}
}

func TestWebcamIdentity(t *testing.T) {
	usb := map[string]string{
		"idVendor": "04f2", "idProduct": "b642",
		"manufacturer": "Sonix Technology Co., Ltd.",
		"product":      "HD Webcam",
		"serial":       "",
		"removable":    "fixed",
	}
	got := webcamIdentity("Integrated Cam: card", usb)
	want := map[string]string{
		"collector": "webcam", "model": "HD Webcam",
		"vendor": "Sonix Technology Co., Ltd.", "connection": "Integrada",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, quer %q", k, got[k], v)
		}
	}
	if _, ok := got["serial"]; ok {
		t.Errorf("serial vazio não deveria entrar")
	}

	// removable=removable → USB; product ausente → cai no name
	got2 := webcamIdentity("Logitech C920", map[string]string{"removable": "removable"})
	if got2["connection"] != "USB" {
		t.Errorf("connection = %q, quer USB", got2["connection"])
	}
	if got2["model"] != "Logitech C920" {
		t.Errorf("model fallback = %q", got2["model"])
	}

	// removable ausente → Desconhecida
	if l := connectionLabel(""); l != "Desconhecida" {
		t.Errorf("connectionLabel(\"\") = %q", l)
	}
}

func TestReadUSBAttrs_TempTree(t *testing.T) {
	root := t.TempDir()
	// device USB (com idVendor/removable) e a interface abaixo dele
	usbDev := filepath.Join(root, "usb1", "1-5")
	iface := filepath.Join(usbDev, "1-5:1.0")
	if err := os.MkdirAll(iface, 0755); err != nil {
		t.Fatal(err)
	}
	write := func(dir, k, v string) {
		if err := os.WriteFile(filepath.Join(dir, k), []byte(v+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(usbDev, "idVendor", "04f2")
	write(usbDev, "idProduct", "b642")
	write(usbDev, "manufacturer", "Sonix Technology Co., Ltd.")
	write(usbDev, "product", "HD Webcam")
	write(usbDev, "removable", "fixed")

	// symlink `device` → interface (como no /sys real)
	videoDir := filepath.Join(root, "video0")
	if err := os.MkdirAll(videoDir, 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(videoDir, "device")
	if err := os.Symlink(iface, link); err != nil {
		t.Fatal(err)
	}

	got := readUSBAttrs(link)
	if got["idVendor"] != "04f2" || got["product"] != "HD Webcam" || got["removable"] != "fixed" {
		t.Fatalf("readUSBAttrs inesperado: %+v", got)
	}
}

func TestWebcamCollect_SysfsRoot(t *testing.T) {
	root := t.TempDir()
	usbDev := filepath.Join(root, "pcidev", "usb1", "1-5")
	iface := filepath.Join(usbDev, "1-5:1.0")
	os.MkdirAll(iface, 0755)
	os.WriteFile(filepath.Join(usbDev, "idVendor"), []byte("04f2\n"), 0644)
	os.WriteFile(filepath.Join(usbDev, "idProduct"), []byte("b642\n"), 0644)
	os.WriteFile(filepath.Join(usbDev, "product"), []byte("HD Webcam\n"), 0644)
	os.WriteFile(filepath.Join(usbDev, "removable"), []byte("fixed\n"), 0644)
	videoDir := filepath.Join(root, "video0")
	os.MkdirAll(videoDir, 0755)
	os.WriteFile(filepath.Join(videoDir, "name"), []byte("HD Webcam: HD Webcam\n"), 0644)
	os.Symlink(iface, filepath.Join(videoDir, "device"))

	w := &Webcam{sysClassVideo: root}
	got, _ := w.Collect(context.Background(), Target{RTSPURL: "rtsp://127.0.0.1:8554/webcam0"})
	if got["collector"] != "webcam" || got["model"] != "HD Webcam" || got["connection"] != "Integrada" {
		t.Fatalf("Collect inesperado: %+v", got)
	}
}
