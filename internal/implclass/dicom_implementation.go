package implclass

import "sync"

type Implementation interface {
	GetClassUID() string
	GetVersion() string
}

type dicomImplementation struct {
	classUID string
	version  string
}

var (
	imp   Implementation
	impMu sync.RWMutex
)

func SetDefaultImplementation() Implementation {
	impMu.Lock()
	defer impMu.Unlock()
	imp = &dicomImplementation{
		classUID: "1.2.826.0.1.3680043.10.90.999",
		version:  "IO-Dicom",
	}
	return imp
}

func SetImplementation(classUID string, version string) Implementation {
	impMu.Lock()
	defer impMu.Unlock()
	imp = &dicomImplementation{
		classUID: classUID,
		version:  version,
	}
	return imp
}

func GetImplementationClassUID() string {
	impMu.RLock()
	i := imp
	impMu.RUnlock()
	if i == nil {
		return SetDefaultImplementation().GetClassUID()
	}
	return i.GetClassUID()
}

func GetImplementationVersion() string {
	impMu.RLock()
	i := imp
	impMu.RUnlock()
	if i == nil {
		return SetDefaultImplementation().GetVersion()
	}
	return i.GetVersion()
}

func (i *dicomImplementation) GetClassUID() string {
	if i.classUID == "" {
		def := SetDefaultImplementation()
		i.classUID = def.GetClassUID()
		i.version = def.GetVersion()
	}
	return i.classUID
}

func (i *dicomImplementation) GetVersion() string {
	if i.classUID == "" {
		def := SetDefaultImplementation()
		i.classUID = def.GetClassUID()
		i.version = def.GetVersion()
	}
	return i.version
}
