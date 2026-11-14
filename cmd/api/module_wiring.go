package main

// wire supplies dependencies used only by the API.
func (m moduleSet) wire(a *Application) {
	m.Site.SetDomainRepository(m.dns.Repos().Domain())
	m.staff.SetServerLogReader(m.Server.Repos().Task())
	m.staff.Service().SetServerDeleter(m.Server.Service())
	m.staff.Service().SetSiteDeleter(m.Site.SiteService())
	m.staff.Service().SetInvitationDeps(m.emailSender, a.config.App.Frontend())
	m.auth.Service().Auth.SetPlatformInviteReader(m.staff.Service())
}
