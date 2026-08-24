package collectors

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/devscope/devscope/internal/core"
)

const dockerImagesFormat = "{{.ID}}\t{{.Repository}}\t{{.Tag}}\t{{.CreatedSince}}\t{{.Size}}"

// CollectDockerImages lists local top-level images (matches what `docker images`
// / Docker Desktop's Images screen shows — no intermediate build layers).
func CollectDockerImages(ctx context.Context) ([]core.Image, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, nil
	}
	out, err := exec.CommandContext(ctx, "docker", "images", "--format", dockerImagesFormat).Output()
	if err != nil {
		return nil, dockerExitErr(err)
	}

	var images []core.Image
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 5 {
			continue
		}
		images = append(images, core.Image{
			ID:         parts[0],
			Repository: parts[1],
			Tag:        parts[2],
			Created:    parts[3],
			Size:       parts[4],
		})
	}
	applyImageComposeLabels(ctx, images)
	return images, nil
}

// applyImageComposeLabels fills ComposeProject/ComposeService from each image's
// own build-time labels — these persist on an image even after a rebuild makes
// it dangling (untagged), which is how the project scope also catches old,
// disk-hogging rebuilds that `docker images` no longer names.
func applyImageComposeLabels(ctx context.Context, images []core.Image) {
	if len(images) == 0 {
		return
	}
	ids := make([]string, len(images))
	for i, img := range images {
		ids[i] = img.ID
	}
	// Plain JSON (not -f template): several base images omit Config.Labels
	// entirely, and docker's template engine errors on `index` into a missing
	// map key instead of returning zero value. json.Unmarshal handles a null
	// field fine (nil map, safe to read from).
	out, err := exec.CommandContext(ctx, "docker", append([]string{"image", "inspect"}, ids...)...).Output()
	if err != nil {
		return
	}
	var details []struct {
		Id     string
		Config struct {
			Labels map[string]string
		}
	}
	if json.Unmarshal(out, &details) != nil {
		return
	}
	byID := make(map[string][2]string, len(details))
	for _, d := range details {
		short := strings.TrimPrefix(d.Id, "sha256:")
		if len(short) > 12 {
			short = short[:12]
		}
		pair := [2]string{d.Config.Labels["com.docker.compose.project"], d.Config.Labels["com.docker.compose.service"]}
		byID[d.Id] = pair
		byID[short] = pair
	}
	for i := range images {
		if pair, ok := byID[images[i].ID]; ok {
			images[i].ComposeProject = pair[0]
			images[i].ComposeService = pair[1]
		}
	}
}

// RemoveDockerImages runs `docker rmi` for the given IDs in one call and
// returns its combined output (useful as a status message either way).
func RemoveDockerImages(ctx context.Context, ids []string, force bool) (string, error) {
	if len(ids) == 0 {
		return "", nil
	}
	args := []string{"rmi"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, ids...)
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func dockerExitErr(err error) error {
	if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
	}
	return err
}
