package liveupdates

import (
	"fmt"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// If a container crashes, and it's been live-updated in the past,
// then it needs to enter a special state to indicate that it
// needs to be rebuilt (because the file system has been reset to the original image).
//
// Eventually, this will be represented by a special state on the LiveUpdateStatus.
func CheckForContainerCrash(state *store.EngineState, name string) {
	mt, ok := state.ManifestTargets[model.ManifestName(name)]
	if !ok {
		return
	}

	ms := mt.State
	if ms.NeedsRebuildFromCrash {
		// We're already aware the pod is crashing.
		return
	}

	runningContainers := store.AllRunningContainers(mt, state)
	if len(runningContainers) == 0 {
		// If there are no running containers, it might mean the containers are
		// being deleted. We don't need to intervene yet.
		return
	}

	hitList := make(map[container.ID]bool, len(ms.LiveUpdatedContainerIDs))
	for cID := range ms.LiveUpdatedContainerIDs {
		hitList[cID] = true
	}
	for _, c := range runningContainers {
		delete(hitList, c.ContainerID)
	}

	if len(hitList) == 0 {
		// The pod is what we expect it to be.
		return
	}

	// There are new running containers that don't match
	// what we live-updated!
	ms.NeedsRebuildFromCrash = true
	ms.LiveUpdatedContainerIDs = container.NewIDSet()

	msg := fmt.Sprintf("Detected a container change for %s. We could be running stale code. Rebuilding and deploying a new image.", ms.Name)
	le := store.NewLogAction(ms.Name, ms.LastBuild().SpanID, logger.WarnLvl, nil, []byte(msg+"\n"))
	state.LogStore.Append(le, state.Secrets)
}
