// Package images guarda as imagens dos produtos: em disco no desenvolvimento e no Amazon S3 em produção.
package images

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var ErrNotFound = errors.New("imagem não encontrada")

type Storage interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Exists(ctx context.Context, key string) (bool, error)
	Delete(ctx context.Context, key string) error
}

// Chaves sempre no formato <tipo>/<nome>.<ext>, sem "..", barras extras ou espaços.
var keyPattern = regexp.MustCompile(`^[a-z0-9_]+/[a-z0-9][a-z0-9-]*\.(png|jpg|webp)$`)

func ValidKey(key string) bool { return keyPattern.MatchString(key) }

// Slug normaliza um texto para uso em nomes de arquivo: "GeForce RTX 3060" -> "geforce-rtx-3060".
func Slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		switch {
		case unicode.Is(unicode.Mn, r): // acentos
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			dash = false
		default:
			if b.Len() > 0 && !dash {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// BuildKey gera a chave de uma imagem enviada pelo admin. O version (ex.: timestamp) muda a URL
// a cada upload, para o navegador não mostrar a imagem antiga do cache.
func BuildKey(partType string, productID int64, name, ext string, version int64) string {
	slug := Slug(name)
	if slug == "" {
		slug = "produto"
	}
	if len(slug) > 80 {
		slug = strings.TrimSuffix(slug[:80], "-")
	}
	return fmt.Sprintf("%s/%d-%s-%s.%s", partType, productID, slug, strconv.FormatInt(version, 36), ext)
}

// ContentType devolve o MIME type a partir da extensão da chave.
func ContentType(key string) string {
	switch path.Ext(key) {
	case ".png":
		return "image/png"
	case ".jpg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	}
	return "application/octet-stream"
}

// Local guarda as imagens em um diretório do disco.
type Local struct{ root string }

func NewLocal(root string) (*Local, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Local{root: root}, nil
}

func (l *Local) path(key string) (string, error) {
	if !ValidKey(key) {
		return "", fmt.Errorf("chave inválida: %q", key)
	}
	return filepath.Join(l.root, filepath.FromSlash(key)), nil
}

func (l *Local) Put(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".upload-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p)
}

func (l *Local) Get(_ context.Context, key string) (io.ReadCloser, error) {
	p, err := l.path(key)
	if err != nil {
		return nil, ErrNotFound
	}
	f, err := os.Open(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	return f, err
}

func (l *Local) Exists(_ context.Context, key string) (bool, error) {
	p, err := l.path(key)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func (l *Local) Delete(_ context.Context, key string) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// Seed envia para o storage as imagens de dir que ainda não existem lá.
// É assim que as imagens dos produtos iniciais chegam ao bucket S3 no primeiro deploy.
func Seed(ctx context.Context, st Storage, dir string) (uploaded int, err error) {
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(rel)
		if !ValidKey(key) {
			slog.Warn("ignorando arquivo de seed com nome inválido", "arquivo", key)
			return nil
		}
		exists, err := st.Exists(ctx, key)
		if err != nil || exists {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return err
		}
		if err := st.Put(ctx, key, f, info.Size(), ContentType(key)); err != nil {
			return fmt.Errorf("enviando %s: %w", key, err)
		}
		uploaded++
		return nil
	})
	return uploaded, err
}
