package runtime

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetRuntimeArgs", func() {
	bin := "strict-supplementalgroups-container-runtime"
	root := "/run/containerd/runc/k8s.io"
	containerId := "48bea6a58de41cdcae1521af1e3849400e498b9535f4d54a29771e8e0c67acf9"
	bundleDir := "/run/containerd/io.containerd.v2.task/k8s.io/" + containerId
	process := "/tmp/runc-process1668049232"

	DescribeTable("command pattern",
		func(args []string, expected *RuntimeArgs) {
			got, err := GetRuntimeArgs(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(BeEquivalentTo(expected))
		},

		Entry(
			"empty options",
			[]string{bin},
			&RuntimeArgs{},
		),

		// containerd
		Entry(
			"[containerd] create",
			[]string{
				bin,
				"--root", root,
				"--log", bundleDir + "/log.json",
				"--log-format", "json",
				"--system-cgroup",
				"create",
				"--bundle", bundleDir,
				"--pid-file", bundleDir + "/init.pid",
				containerId,
			},
			&RuntimeArgs{
				Command:     CommandCreate,
				ContainerId: containerId,
				Options: RuntimeOpts{
					Root:      root,
					Log:       bundleDir + "/log.json",
					LogFormat: "json",
					PidFile:   bundleDir + "/init.pid",
					Bundle:    bundleDir,
				},
			},
		),
		Entry(
			"[containerd] start",
			[]string{
				bin,
				"--root", root,
				"--log", bundleDir + "/log.json",
				"--log-format", "json",
				"--system-cgroup",
				"start",
				containerId,
			},
			&RuntimeArgs{
				Command:     CommandStart,
				ContainerId: containerId,
				Options: RuntimeOpts{
					Root:      root,
					Log:       bundleDir + "/log.json",
					LogFormat: "json",
				},
			},
		),
		Entry(
			"[containerd] exec",
			[]string{
				bin,
				"--root", root,
				"--log", bundleDir + "/log.json",
				"--log-format", "json",
				"--system-cgroup",
				"exec",
				"--process", "/tmp/runc-process1668049232",
				"--console-socket", "/tmp/pty291649257/pty.sock", "--detach",
				"--pid-file", bundleDir + "/init.pid",
				containerId,
			},
			&RuntimeArgs{
				Command:     CommandExec,
				ContainerId: containerId,
				Options: RuntimeOpts{
					Root:      root,
					Log:       bundleDir + "/log.json",
					LogFormat: "json",
					PidFile:   bundleDir + "/init.pid",
					Process:   process,
				},
			},
		),

		// cri-o
		Entry(
			"[cri-o] create",
			[]string{
				bin,
				"--root=" + root,
				"--system-cgroup",
				"create",
				"--bundle", bundleDir,
				"--pid-file", bundleDir + "/pidfile",
				containerId,
			},
			&RuntimeArgs{
				Command:     CommandCreate,
				ContainerId: containerId,
				Options: RuntimeOpts{
					Root:    root,
					PidFile: bundleDir + "/pidfile",
					Bundle:  bundleDir,
				},
			},
		),

		Entry(
			"[cri-o] start",
			[]string{
				bin,
				"--root", root,
				"--system-cgroup",
				"start",
				containerId,
			},
			&RuntimeArgs{
				Command:     CommandStart,
				ContainerId: containerId,
				Options: RuntimeOpts{
					Root: root,
				},
			},
		),

		Entry(
			"[cri-o] exec",
			[]string{
				bin,
				"--root", root,
				"--system-cgroup",
				"exec",
				"--process", process,
				containerId,
			},
			&RuntimeArgs{
				Command:     CommandExec,
				ContainerId: containerId,
				Options: RuntimeOpts{
					Root:    root,
					Process: process,
				},
			},
		),
	)
})
