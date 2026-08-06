package util

import (
	"archive/tar"
	"compress/gzip"
	"github.com/schollz/progressbar/v3"
	"io"
	"os"
	"path/filepath"
)

func UnTarGz(src string, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	bar := progressbar.Default(-1, "Extracting tar.gz file")

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}

		_ = bar.Add(1)
	}

	return nil
}
