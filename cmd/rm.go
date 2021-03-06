package cmd

import (
	"sort"

	"github.com/spf13/cobra"

	"github.com/fromanirh/pack8s/cmd/cmdutil"

	"github.com/fromanirh/pack8s/iopodman"

	"github.com/fromanirh/pack8s/internal/pkg/podman"
)

type rmOptions struct {
	prune bool
}

// NewRemoveCommand returns command to remove the cluster
func NewRemoveCommand() *cobra.Command {
	flags := &rmOptions{}
	rm := &cobra.Command{
		Use:   "rm",
		Short: "rm deletes all traces of a cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			return remove(cmd, flags)
		},
		Args: cobra.NoArgs,
	}

	rm.Flags().BoolVarP(&flags.prune, "prune", "P", false, "prune removes unused volumes on the host")

	return rm
}

type containerList []iopodman.Container

func (cl containerList) Len() int {
	return len(cl)
}

func (cl containerList) Less(i, j int) bool {
	contA := cl[i]
	contB := cl[j]
	genA := contA.Labels[podman.LabelGeneration]
	genB := contB.Labels[podman.LabelGeneration]
	if genA != "" && genB != "" {
		// CAVEAT! we want the latest generation first, so we swap the condition
		return genA > genB
	}
	return false // do not change the ordering
}

func (cl containerList) Swap(i, j int) {
	cl[i], cl[j] = cl[j], cl[i]
}

func remove(cmd *cobra.Command, rmOpts *rmOptions) error {
	cOpts, err := cmdutil.GetCommonOpts(cmd)
	if err != nil {
		return err
	}

	hnd, log, err := cOpts.GetHandle()
	if err != nil {
		return err
	}

	containers, err := hnd.GetPrefixedContainers(cOpts.Prefix)
	if err != nil {
		return err
	}

	sort.Sort(containerList(containers))

	force := true
	removeVolumes := true

	log.Infof("bringing cluster down (containers=%d)", len(containers))

	for _, cont := range containers {
		log.Noticef("stopping container: %s", cont.Names)

		_, err = hnd.StopContainer(cont.Id, 5) // TODO
		if err != nil {
			return err
		}

		log.Noticef("removing container: %s", cont.Names)
		_, err = hnd.RemoveContainer(cont, force, removeVolumes)
		if err != nil {
			return err
		}
	}

	volumes, err := hnd.GetPrefixedVolumes(cOpts.Prefix)
	if err != nil {
		return err
	}

	if len(volumes) > 0 {
		log.Infof("cleaning cluster (volumes=%d)", len(volumes))
		err = hnd.RemoveVolumes(volumes)
		if err != nil {
			return err
		}
	}

	if rmOpts.prune {
		err = hnd.PruneVolumes()
	}

	log.Infof("cluster removed err=%s", err)
	return err
}
