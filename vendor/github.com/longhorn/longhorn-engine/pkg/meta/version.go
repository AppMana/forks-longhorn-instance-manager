package meta

import "runtime"

const (
	// CLIAPIVersion used to communicate with user e.g. longhorn-manager
	CLIAPIVersion = 12
	// CLIAPIMinVersion is the minimum CLI API version that the engine supports. If the CLI API version of the client is lower than this, the engine will can not be used.
	// such as longhorn-manager, CLIAPIVersion 8 is introduced at 1.5.0, and CLIAPIMinVersion 8 is introduced at 1.6.0,
	// which means longhorn-manager lower than 1.5.0 can not work with engine 1.6.0 or later.
	CLIAPIMinVersion = 8

	// ControllerAPIVersion used to communicate with instance-manager
	ControllerAPIVersion    = 6
	ControllerAPIMinVersion = 4

	// DataFormatVersion used by the Replica to store data
	DataFormatVersion    = 1
	DataFormatMinVersion = 1
)

// Following variables are filled in by main.go
var (
	Version   string
	GitCommit string
	BuildDate string
)

type VersionOutput struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	BuildDate string `json:"buildDate"`

	CLIAPIVersion           int                `json:"cliAPIVersion"`
	CLIAPIMinVersion        int                `json:"cliAPIMinVersion"`
	ControllerAPIVersion    int                `json:"controllerAPIVersion"`
	ControllerAPIMinVersion int                `json:"controllerAPIMinVersion"`
	DataFormatVersion       int                `json:"dataFormatVersion"`
	DataFormatMinVersion    int                `json:"dataFormatMinVersion"`
	Capabilities            EngineCapabilities `json:"capabilities"`
}

type EngineCapabilities struct {
	Controller []string `json:"controller"`
	Replica    []string `json:"replica"`
	Frontend   []string `json:"frontend"`
	Disk       []string `json:"disk"`
}

const (
	CapabilityV1               = "engine:v1"
	CapabilityRWO              = "access-mode:rwo"
	CapabilityRWOP             = "access-mode:rwop"
	CapabilityRWX              = "access-mode:rwx"
	CapabilityBestEffort       = "data-locality:best-effort"
	CapabilityStrictLocal      = "data-locality:strict-local"
	CapabilityEncryption       = "volume:encryption"
	CapabilityBackingImage     = "volume:backing-image"
	CapabilityFilesystemFreeze = "snapshot:filesystem-freeze"
	CapabilityISCSI            = "frontend:iscsi"
	CapabilityLiveUpgrade      = "frontend:live-upgrade"
	CapabilityExt4             = "workload-fs:ext4"
	CapabilityXFS              = "workload-fs:xfs"
	CapabilityNTFS             = "workload-fs:ntfs"
	CapabilityReFS             = "workload-fs:refs"
	CapabilitySparse           = "replica-store:sparse"
	CapabilityReplicaNTFS      = "replica-store:ntfs"
	CapabilityReplicaReFS      = "replica-store:refs"
)

func GetCapabilities() EngineCapabilities {
	base := []string{
		CapabilityV1,
		CapabilityRWO,
		CapabilityRWOP,
		CapabilityBestEffort,
	}
	if runtime.GOOS == "windows" {
		return EngineCapabilities{
			Controller: append([]string{}, base...),
			Replica:    append([]string{}, base...),
			Frontend: []string{
				CapabilityISCSI,
				CapabilityLiveUpgrade,
				CapabilityNTFS,
				CapabilityReFS,
			},
			Disk: []string{
				CapabilitySparse,
				CapabilityReplicaNTFS,
				CapabilityReplicaReFS,
			},
		}
	}

	base = append(base, CapabilityBackingImage)
	linux := append(base,
		CapabilityRWX,
		CapabilityStrictLocal,
		CapabilityEncryption,
		CapabilityFilesystemFreeze,
	)
	return EngineCapabilities{
		Controller: append([]string{}, linux...),
		Replica:    append([]string{}, linux...),
		Frontend: []string{
			CapabilityISCSI,
			CapabilityLiveUpgrade,
			CapabilityExt4,
			CapabilityXFS,
		},
		Disk: []string{CapabilitySparse},
	}
}

func GetVersion() VersionOutput {
	return VersionOutput{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,

		CLIAPIVersion:           CLIAPIVersion,
		CLIAPIMinVersion:        CLIAPIMinVersion,
		ControllerAPIVersion:    ControllerAPIVersion,
		ControllerAPIMinVersion: ControllerAPIMinVersion,
		DataFormatVersion:       DataFormatVersion,
		DataFormatMinVersion:    DataFormatMinVersion,
		Capabilities:            GetCapabilities(),
	}
}
