package collectors

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ImageConfig is the subset of image config used to seed compose YAML.
type ImageConfig struct {
	ExposedPorts map[string]struct{}
	Env          []string
	Volumes      map[string]struct{}
}

type registryTokenResp struct {
	Token       string `json:"token"`
	AccessToken string `json:"access_token"`
}

type dockerManifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Config        struct {
		Digest string `json:"digest"`
	} `json:"config"`
	Manifests []struct {
		Digest   string `json:"digest"`
		Platform struct {
			Architecture string `json:"architecture"`
			OS           string `json:"os"`
		} `json:"platform"`
	} `json:"manifests"`
}

type imageConfigBlob struct {
	Config struct {
		ExposedPorts map[string]json.RawMessage `json:"ExposedPorts"`
		Env          []string                   `json:"Env"`
		Volumes      map[string]json.RawMessage `json:"Volumes"`
	} `json:"config"`
}

// InspectImageConfig fetches container config for a public Docker Hub image (anonymous).
func InspectImageConfig(image string) (ImageConfig, error) {
	repo, tag := splitRegistryImage(image)
	if repo == "" {
		return ImageConfig{}, fmt.Errorf("imagem inválida")
	}
	client := &http.Client{Timeout: 8 * time.Second}
	token, err := registryPullToken(client, repo)
	if err != nil {
		return ImageConfig{}, err
	}
	digest, err := registryConfigDigest(client, token, repo, tag)
	if err != nil {
		return ImageConfig{}, err
	}
	return registryFetchConfig(client, token, repo, digest)
}

func splitRegistryImage(image string) (repo, tag string) {
	image = strings.TrimSpace(image)
	if image == "" {
		return "", ""
	}
	if i := strings.Index(image, "@"); i >= 0 {
		image = image[:i]
	}
	tag = "latest"
	// registry/host has a dot or colon before first slash — skip for docker hub short names
	slash := strings.LastIndex(image, "/")
	colon := strings.LastIndex(image, ":")
	if colon > slash {
		tag = image[colon+1:]
		image = image[:colon]
	}
	image = strings.TrimPrefix(image, "docker.io/")
	if !strings.Contains(image, "/") {
		image = "library/" + image
	}
	return image, tag
}

func registryPullToken(client *http.Client, repo string) (string, error) {
	u := "https://auth.docker.io/token?service=registry.docker.io&scope=repository:" + repo + ":pull"
	resp, err := client.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry auth HTTP %d", resp.StatusCode)
	}
	var body registryTokenResp
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	tok := body.Token
	if tok == "" {
		tok = body.AccessToken
	}
	if tok == "" {
		return "", fmt.Errorf("registry token vazio")
	}
	return tok, nil
}

func registryConfigDigest(client *http.Client, token, repo, tag string) (string, error) {
	u := "https://registry-1.docker.io/v2/" + repo + "/manifests/" + tag
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.oci.image.index.v1+json",
	}, ", "))
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("manifest HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	var man dockerManifest
	if err := json.Unmarshal(raw, &man); err != nil {
		return "", err
	}
	if man.Config.Digest != "" {
		return man.Config.Digest, nil
	}
	// manifest list / index: pick linux/amd64 then first
	var pick string
	for _, m := range man.Manifests {
		if m.Platform.OS == "linux" && (m.Platform.Architecture == "amd64" || m.Platform.Architecture == "x86_64") {
			pick = m.Digest
			break
		}
	}
	if pick == "" && len(man.Manifests) > 0 {
		pick = man.Manifests[0].Digest
	}
	if pick == "" {
		return "", fmt.Errorf("manifest sem config")
	}
	return registryConfigDigest(client, token, repo, pick)
}

func registryFetchConfig(client *http.Client, token, repo, digest string) (ImageConfig, error) {
	u := "https://registry-1.docker.io/v2/" + repo + "/blobs/" + digest
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return ImageConfig{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return ImageConfig{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ImageConfig{}, fmt.Errorf("config blob HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return ImageConfig{}, err
	}
	return ParseImageConfigJSON(raw)
}

// ParseImageConfigJSON extracts ExposedPorts/Env/Volumes from an image config blob.
func ParseImageConfigJSON(raw []byte) (ImageConfig, error) {
	var blob imageConfigBlob
	if err := json.Unmarshal(raw, &blob); err != nil {
		return ImageConfig{}, err
	}
	out := ImageConfig{
		Env: append([]string(nil), blob.Config.Env...),
	}
	if len(blob.Config.ExposedPorts) > 0 {
		out.ExposedPorts = make(map[string]struct{}, len(blob.Config.ExposedPorts))
		for k := range blob.Config.ExposedPorts {
			out.ExposedPorts[k] = struct{}{}
		}
	}
	if len(blob.Config.Volumes) > 0 {
		out.Volumes = make(map[string]struct{}, len(blob.Config.Volumes))
		for k := range blob.Config.Volumes {
			out.Volumes[k] = struct{}{}
		}
	}
	return out, nil
}
