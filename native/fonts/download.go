package fonts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"gioui.org/font"
	"gioui.org/font/opentype"
)

// RemoteFile is one font file lich downloads rather than ships: where it is,
// how many bytes and which SHA-256 it must have, and the weight it draws.
type RemoteFile struct {
	Name   string
	URL    string
	Size   int64
	SHA256 string
	Weight font.Weight
}

// notoCJKCommit pins notofonts/noto-cjk at the commit that last touched
// Sans/SubsetOTF/SC (Noto Sans CJK 2.004), so the files behind the URLs never
// change under the hashes. JP and KR are read at the same commit.
const notoCJKCommit = "165c01b46ea533872e002e0785ff17e44f6d97d8"

const (
	notoCJKBase = "https://raw.githubusercontent.com/notofonts/noto-cjk/" + notoCJKCommit + "/Sans/SubsetOTF/"
	notoSCBase  = notoCJKBase + "SC/"
	notoJPBase  = notoCJKBase + "JP/"
	notoKRBase  = notoCJKBase + "KR/"
)

// NotoSansSC is Noto Sans SC (SIL Open Font License 1.1) in the interface's
// weights, as static instances: Gio picks a weight by face, and google/fonts
// only has the family as a variable font. Sizes are GitHub's contents API's
// for that commit, hashes sha256sum's of the files downloaded from these URLs
// on 2026-10-16.
var NotoSansSC = []RemoteFile{
	{"NotoSansSC-Regular.otf", notoSCBase + "NotoSansSC-Regular.otf", 8331336, "faa6c9df652116dde789d351359f3d7e5d2285a2b2a1f04a2d7244df706d5ea9", font.Normal},
	{"NotoSansSC-Medium.otf", notoSCBase + "NotoSansSC-Medium.otf", 8345436, "7633f5a016d4dd95e685a69633d818aabc4644c4b08e26bd35b1b30c45ed5dda", font.Medium},
	{"NotoSansSC-Bold.otf", notoSCBase + "NotoSansSC-Bold.otf", 8543168, "c6cb5a93abaa9edc8ee7463b7ebb7f42d618d40e6ed2f7a5371c97b0b64767c0", font.Bold},
}

// NotoSansJP and NotoSansKR are NotoSansSC's Japanese and Korean siblings,
// pinned the same way: sizes from GitHub's contents API, hashes of the files
// downloaded on 2026-10-16.
var NotoSansJP = []RemoteFile{
	{"NotoSansJP-Regular.otf", notoJPBase + "NotoSansJP-Regular.otf", 4533028, "dff723ba59d57d136764a04b9b2d03205544f7cd785a711442d6d2d085ac5073", font.Normal},
	{"NotoSansJP-Medium.otf", notoJPBase + "NotoSansJP-Medium.otf", 4549132, "f396a3b57256e4515be9cb41f7aac54766d654890082a9f1b5c2451b5c093d8a", font.Medium},
	{"NotoSansJP-Bold.otf", notoJPBase + "NotoSansJP-Bold.otf", 4656448, "1b0edfb500b73a4fa8a4fcaae1bbbd403994e08e73e3e0da37e70d3853f42c5f", font.Bold},
}

var NotoSansKR = []RemoteFile{
	{"NotoSansKR-Regular.otf", notoKRBase + "NotoSansKR-Regular.otf", 4644748, "69975a0ac8472717870aefeab0a4d52739308d90856b9955313b2ad5e0148d68", font.Normal},
	{"NotoSansKR-Medium.otf", notoKRBase + "NotoSansKR-Medium.otf", 4669816, "b46988ef13e8bac08f3933af686eaf770972994f9b6d335be0184d60169b5431", font.Medium},
	{"NotoSansKR-Bold.otf", notoKRBase + "NotoSansKR-Bold.otf", 4816044, "5a6ceb287ed2fc6cfc6213144ebea68cbd94b20fc9eb873d8486493bf02d9bda", font.Bold},
}

// NotoSansFor is the set lich downloads for the CJK faces of the interface
// in language, and its family's name: Japanese's and Korean's own, Simplified
// Chinese's for every other language, as InstalledHan picks.
func NotoSansFor(language string) (family string, files []RemoteFile) {
	switch cjkOf(language) {
	case japanese:
		return "Noto Sans JP", NotoSansJP
	case korean:
		return "Noto Sans KR", NotoSansKR
	}
	return "Noto Sans SC", NotoSansSC
}

// TotalSize is how many bytes files download.
func TotalSize(files []RemoteFile) int64 {
	var total int64
	for _, f := range files {
		total += f.Size
	}
	return total
}

// DownloadDir is where lich keeps the fonts it downloads: its own cache,
// never a system font folder.
func DownloadDir() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "lich", "fonts"), nil
}

// Fetch downloads each of files into dir, skipping the ones already there
// with the right hash, and reports the bytes done out of TotalSize. A file
// lands under its name only once its whole body hashed right; a canceled
// ctx, an HTTP error or a wrong hash leaves no file behind and is returned.
func Fetch(ctx context.Context, files []RemoteFile, dir string, progress func(done, total int64)) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	total := TotalSize(files)
	var done int64
	for _, f := range files {
		path := filepath.Join(dir, f.Name)
		if data, err := os.ReadFile(path); err == nil && verify(f, data) == nil {
			done += f.Size
			progress(done, total)
			continue
		}
		base := done
		err := fetchOne(ctx, f, path, func(n int64) { progress(base+n, total) })
		if err != nil {
			return err
		}
		done += f.Size
	}
	return nil
}

func fetchOne(ctx context.Context, f RemoteFile, path string, progress func(n int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", f.Name, resp.Status)
	}
	part := path + ".part"
	out, err := os.Create(part)
	if err != nil {
		return err
	}
	defer os.Remove(part)
	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(out, hash, &progressWriter{report: progress}), resp.Body)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("download %s: %w", f.Name, err)
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != f.SHA256 {
		return fmt.Errorf("download %s: SHA-256 is %s, want %s", f.Name, got, f.SHA256)
	}
	return os.Rename(part, path)
}

// progressWriter reports the running count of bytes written through it.
type progressWriter struct {
	n      int64
	report func(n int64)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	p.n += int64(len(b))
	p.report(p.n)
	return len(b), nil
}

func verify(f RemoteFile, data []byte) error {
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != f.SHA256 {
		return fmt.Errorf("%s: SHA-256 is %s, want %s", f.Name, got, f.SHA256)
	}
	return nil
}

// LoadDownloaded parses files from dir, registered under Han in every
// interface weight, checking each hash again. A file missing from dir is an error that errors.Is
// fs.ErrNotExist: nothing was downloaded yet.
func LoadDownloaded(files []RemoteFile, dir string) ([]font.FontFace, error) {
	faces := make([]font.FontFace, 0, len(files))
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f.Name))
		if err != nil {
			return nil, err
		}
		if err := verify(f, data); err != nil {
			return nil, err
		}
		face, err := opentype.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", f.Name, err)
		}
		faces = append(faces, font.FontFace{Font: font.Font{Typeface: Han, Weight: f.Weight}, Face: face})
	}
	return coverWeights(faces), nil
}
