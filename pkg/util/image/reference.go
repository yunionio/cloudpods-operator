package image

import (
	_ "crypto/sha256"
	_ "crypto/sha512"
	"fmt"
	"strings"

	"github.com/docker/distribution/reference"
)

type ImageReference struct {
	Domain     string
	Repository string
	Image      string
	Tag        string
	Digest     string
}

func ParseImageReference(img string) (*ImageReference, error) {
	ref, err := reference.Parse(img)
	if err != nil {
		return nil, err
	}
	imgRef := new(ImageReference)

	if named, ok := ref.(reference.Named); ok {
		repo := named.Name()
		domain, _ := reference.SplitHostname(named)
		imgRef.Domain = domain
		parts := strings.Split(repo, "/")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid named: %s", repo)
		}
		imgRef.Repository = strings.Join(parts[0:len(parts)-1], "/")
		imgRef.Image = parts[len(parts)-1]
	}

	if tagged, ok := ref.(reference.Tagged); ok {
		imgRef.Tag = tagged.Tag()
	}

	if digested, ok := ref.(reference.Digested); ok {
		d := digested.Digest()
		imgRef.Digest = d.String()
	}

	if imgRef.Tag == "" && imgRef.Digest == "" {
		imgRef.Tag = "latest"
	}
	return imgRef, nil
}
