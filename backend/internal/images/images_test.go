package images

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidKey(t *testing.T) {
	valid := []string{"cpu/amd-ryzen-9-5900x.webp", "graphics_card/12-rtx.png", "ram/a.jpg"}
	invalid := []string{
		"../etc/passwd", "cpu/../../x.png", "/cpu/x.png", "cpu/x.exe", "cpu/X.png",
		"cpu/sub/x.png", "cpu/nome com espaco.png", "cpu/.png", "", "cpu/x.png/",
	}
	for _, k := range valid {
		if !ValidKey(k) {
			t.Errorf("deveria ser válida: %q", k)
		}
	}
	for _, k := range invalid {
		if ValidKey(k) {
			t.Errorf("deveria ser inválida: %q", k)
		}
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"GeForce RTX 3060":     "geforce-rtx-3060",
		"  Placa   de Vídeo! ": "placa-de-video",
		"K70 RGB MK.2":         "k70-rgb-mk-2",
		"../../etc/passwd":     "etc-passwd",
		"Ação & Promoção":      "acao-promocao",
		"":                     "",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, esperado %q", in, got, want)
		}
	}
}

func TestBuildKeyIsAlwaysValid(t *testing.T) {
	for _, name := range []string{"RTX 4090", "!!!", strings.Repeat("nome muito longo ", 20)} {
		key := BuildKey("graphics_card", 42, name, "webp", 1730000000)
		if !ValidKey(key) {
			t.Errorf("BuildKey gerou chave inválida para %q: %q", name, key)
		}
	}
	if got := BuildKey("cpu", 7, "Ryzen 7", "png", 36); got != "cpu/7-ryzen-7-10.png" {
		t.Errorf("BuildKey = %q", got)
	}
}

func TestContentType(t *testing.T) {
	if ContentType("a/b.webp") != "image/webp" || ContentType("a/b.png") != "image/png" || ContentType("a/b.jpg") != "image/jpeg" {
		t.Fatal("content type incorreto")
	}
}

func TestLocalStorageRoundTrip(t *testing.T) {
	ctx := context.Background()
	st, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	key := "cpu/teste.png"

	if ok, _ := st.Exists(ctx, key); ok {
		t.Fatal("não deveria existir ainda")
	}
	if err := st.Put(ctx, key, bytes.NewReader([]byte("conteudo")), 8, "image/png"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := st.Exists(ctx, key); !ok {
		t.Fatal("deveria existir")
	}
	rc, err := st.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(rc)
	rc.Close()
	if string(body) != "conteudo" {
		t.Fatalf("conteúdo = %q", body)
	}
	if err := st.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Get(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("esperava ErrNotFound, veio %v", err)
	}
	if err := st.Delete(ctx, key); err != nil {
		t.Fatalf("delete de arquivo ausente não deveria falhar: %v", err)
	}
}

func TestLocalStorageRejectsTraversal(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	st, _ := NewLocal(filepath.Join(root, "imgs"))
	if err := st.Put(ctx, "../fora.png", bytes.NewReader(nil), 0, ""); err == nil {
		t.Fatal("Put deveria rejeitar path traversal")
	}
	if _, err := os.Stat(filepath.Join(root, "fora.png")); err == nil {
		t.Fatal("arquivo foi gravado fora do diretório")
	}
	if _, err := st.Get(ctx, "../../etc/passwd"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get deveria responder ErrNotFound, veio %v", err)
	}
}

func TestSeedUploadsOnlyMissingFiles(t *testing.T) {
	ctx := context.Background()
	src := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(src, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}
	write("cpu/a.webp", "a")
	write("ram/b.webp", "b")
	write("ram/Nome Invalido.png", "x")

	st, _ := NewLocal(t.TempDir())
	n, err := Seed(ctx, st, src)
	if err != nil || n != 2 {
		t.Fatalf("primeiro seed: n=%d err=%v", n, err)
	}
	n, err = Seed(ctx, st, src)
	if err != nil || n != 0 {
		t.Fatalf("segundo seed deveria ser vazio: n=%d err=%v", n, err)
	}
}
