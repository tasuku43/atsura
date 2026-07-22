package processormanifest

import (
	"fmt"
	"reflect"
)

const pinnedReleaseDownload = "https://github.com/rtk-ai/rtk/releases/download/v0.43.0/"

// PinnedManifest returns a fresh copy of the one processor provenance contract
// accepted by native repository tooling. It is code-owned so changing only the
// repository manifest cannot redirect a network fetch to another artifact.
func PinnedManifest() Manifest {
	return Manifest{
		SchemaVersion: SchemaVersion,
		Processors: []Processor{{
			ContractID:     "atsura.output.rtk_go_test_pass.v1",
			Kind:           "atsura.processor.rtk",
			Version:        "0.43.0",
			UpstreamCommit: "5a7880d404db8364d602f2ecdc41dd790f64013f",
			ReleaseURL:     "https://github.com/rtk-ai/rtk/releases/tag/v0.43.0",
			Checksums: Checksums{
				URL:    pinnedReleaseDownload + "checksums.txt",
				SHA256: "b7f973a9693b0cb3de894ec71f74003992080cabcd5b039b9510ed3b299ed5bc",
			},
			License: License{
				SPDX:   "Apache-2.0",
				URL:    "https://raw.githubusercontent.com/rtk-ai/rtk/5a7880d404db8364d602f2ecdc41dd790f64013f/LICENSE",
				SHA256: "4044ade9c21d8b084d3d16a03375cf3b7e166b946a327bb37a3fbbdb53287cfd",
			},
			Notice:       Notice{Status: "absent_upstream"},
			Distribution: "external_user_supplied_not_bundled",
			SBOMReview:   "not_provided_external_dependency",
			Artifacts: []Artifact{
				{
					Target: "linux/amd64", ArchiveName: "rtk-x86_64-unknown-linux-musl.tar.gz",
					ArchiveURL: pinnedReleaseDownload + "rtk-x86_64-unknown-linux-musl.tar.gz", ArchiveSHA256: "ff8a1e7766496e175291a85aeca1dc97c9ff6df33e51e5893d1fbc78fea2a609", ArchiveSize: 4460416,
					BinaryMember: "rtk", BinarySHA256: "f160611f3baee17fe4eb3a04c56a8bc3d15fec4274d8838016088d4776c6f628", BinarySize: 10083968,
				},
				{
					Target: "linux/arm64", ArchiveName: "rtk-aarch64-unknown-linux-gnu.tar.gz",
					ArchiveURL: pinnedReleaseDownload + "rtk-aarch64-unknown-linux-gnu.tar.gz", ArchiveSHA256: "5519f7ca12e5c143a609f0d28a0a77b97413a8dce31c2681f1a41c24519a8731", ArchiveSize: 4087098,
					BinaryMember: "rtk", BinarySHA256: "86bd2badb697e41fa4fae805ed1a42d9b2495600260918d6ba9c148bc40013cf", BinarySize: 8544624,
				},
				{
					Target: "darwin/amd64", ArchiveName: "rtk-x86_64-apple-darwin.tar.gz",
					ArchiveURL: pinnedReleaseDownload + "rtk-x86_64-apple-darwin.tar.gz", ArchiveSHA256: "a85f60e2637811be68366208b8d8b9c5ba1b748cb5df4477ab20cd73d3c5d9f8", ArchiveSize: 4139835,
					BinaryMember: "rtk", BinarySHA256: "22adaa27b3fd6d8906159ba3ff7ca8346e914df112408bcc7a88cda30a3a6107", BinarySize: 9006316,
				},
				{
					Target: "darwin/arm64", ArchiveName: "rtk-aarch64-apple-darwin.tar.gz",
					ArchiveURL: pinnedReleaseDownload + "rtk-aarch64-apple-darwin.tar.gz", ArchiveSHA256: "8a17e49acbd378997eb21d0eb6f7f861111f35b4fc9b1c74edf4c7448e576c65", ArchiveSize: 3759961,
					BinaryMember: "rtk", BinarySHA256: "2dab449f32ea744c30b02a3ef9806e3e7d3b356a145332f3f2aaabb5ea48edee", BinarySize: 7763408,
				},
			},
		}},
	}
}

// ValidatePinned requires every manifest value and artifact order to match the
// code-owned ADR 0012 contract exactly.
func (m Manifest) ValidatePinned() error {
	if !reflect.DeepEqual(m, PinnedManifest()) {
		return fmt.Errorf("processor manifest does not match the exact ADR 0012 pinned contract")
	}
	return nil
}

// LoadPinned performs strict structural loading and then requires the exact
// code-owned provenance before a caller may perform network or execution I/O.
func LoadPinned(repositoryRoot string) (Manifest, error) {
	manifest, err := Load(repositoryRoot)
	if err != nil {
		return Manifest{}, err
	}
	if err := manifest.ValidatePinned(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}
