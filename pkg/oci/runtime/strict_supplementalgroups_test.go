package runtime

import (
	"sort"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	zlog "github.com/rs/zerolog/log"

	"github.com/opencontainers/runtime-spec/specs-go"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("enforceSupplementalGroupsOnProcessSpec", func() {
	uid := uint32(1000)
	gid := uint32(1000)
	r := strictSupplementalGroupsRuntime{}

	testFunc := func(
		additionalGids []uint32,
		supplmentalGroups []int64,
		fsGroup *int64,
		expectEnforced bool,
		expectedAdditionalGids []uint32,
	) {
		processSpec := specs.Process{
			User: specs.User{
				UID:            uid,
				GID:            gid,
				AdditionalGids: additionalGids,
			},
		}

		pod := corev1.Pod{
			Spec: corev1.PodSpec{
				SecurityContext: &corev1.PodSecurityContext{
					RunAsUser:          pointer.Int64(int64(uid)),
					RunAsGroup:         pointer.Int64(int64(gid)),
					SupplementalGroups: supplmentalGroups,
					FSGroup:            fsGroup,
				},
			},
		}

		enforced := r.enforceSupplementalGroupsOnProcessSpec(zlog.Logger, &processSpec, &pod)
		Expect(enforced).To(Equal(expectEnforced))
		sort.Slice(processSpec.User.AdditionalGids, func(i, j int) bool {
			return processSpec.User.AdditionalGids[i] < processSpec.User.AdditionalGids[j]
		})
		sort.Slice(expectedAdditionalGids, func(i, j int) bool {
			return expectedAdditionalGids[i] < expectedAdditionalGids[j]
		})
		Expect(processSpec.User.AdditionalGids).To(BeEquivalentTo(expectedAdditionalGids))
	}

	When("additionalGids ⊆ (supplementalGroups ∪ fsGroup)", func() {
		Context("No enforcement performed", func() {
			DescribeTable(
				"test",
				testFunc,
				Entry("additionalGid=∅,supplementalGroups=∅,fsGroup=∅", nil, nil, nil, false, nil),
				Entry("additionalGid=∅,supplementalGroups=∅,fsGroup!=∅", nil, nil, pointer.Int64(10000), false, nil),
				Entry("additionalGid=∅,supplementalGroups!=∅,fsGroup=∅", nil, []int64{10000}, nil, false, nil),
				Entry("additionalGid=∅,supplementalGroups!=∅,fsGroup!=∅", nil, []int64{10000}, pointer.Int64(10001), false, nil),
				// Skipped "additionalGid!=∅,supplementalGroups=∅,fsGroup=∅" because it is impossible to satisfy additionalGids ⊆ (supplementalGroups ∪ fsGroup)
				Entry("additionalGid!=∅,supplementalGroups=∅,fsGroup!=∅", []uint32{10000}, nil, pointer.Int64(10000), false, []uint32{10000}),
				Entry("additionalGid!=∅,supplementalGroups!=∅,fsGroup=∅", []uint32{10000}, []int64{10000}, nil, false, []uint32{10000}),
				Entry("additionalGid!=∅,supplementalGroups!=∅,fsGroup!=∅", []uint32{10000}, []int64{10000}, pointer.Int64(10001), false, []uint32{10000}),
			)
		})
	})

	When("!(additionalGids ⊆ (supplementalGroups ∪ fsGroup))", func() {
		Context("Enforcement performed", func() {
			DescribeTable(
				"test",
				testFunc,
				// Skipped "additionalGid=∅" cases because it is impossible to satisfy !(additionalGids ⊆ (supplementalGroups ∪ fsGroup))
				Entry("additionalGid!=∅,supplementalGroups=∅,fsGroup=∅", []uint32{20000, 10000}, nil, nil, true, []uint32{}),
				Entry("additionalGid!=∅,supplementalGroups=∅,fsGroup!=∅", []uint32{20000, 10000}, nil, pointer.Int64(20000), true, []uint32{20000}),
				Entry("additionalGid!=∅,supplementalGroups!=∅,fsGroup=∅", []uint32{20000, 10000}, []int64{20000}, nil, true, []uint32{20000}),
				Entry("additionalGid!=∅,supplementalGroups!=∅,fsGroup!=∅", []uint32{20000, 10000, 20001}, []int64{20000, 30000}, pointer.Int64(20001), true, []uint32{20000, 20001}),
			)
		})
	})
})
