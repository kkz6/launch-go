package dto

// CertificateUsage names a single resource that references a stored
// certificate. Returned by /usages and embedded in ErrInUse when the
// user attempts to delete a referenced cert.
type CertificateUsage struct {
	Kind string `json:"kind"` // "site" | "docker_domain"
	ID   string `json:"id"`   // the referencing row's id
	Name string `json:"name"` // human-readable label (site.address, domain.host)
}
