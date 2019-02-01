package release

type HelmRelease struct {
}

func NewHelmRelease(name, version, manifest string) *HelmRelease {

	return &HelmRelease{}
}

func (h *HelmRelease) Types() (gvk []string) {
	return nil
}

func (h *HelmRelease) Resources() (keys []string) {
	return nil
}

//Diff compares all resources found in the helm release with kubernetes API
func (h *HelmRelease) Diff() (string, error) {

	return "", nil

}

//DiffResource compares a single resource from the manifest with the kubernetes API
func (h *HelmRelease) DiffResource(resource interface{}) (string, error) {
	return "", nil

}
