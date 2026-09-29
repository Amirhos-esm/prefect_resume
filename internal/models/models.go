package models

type Language struct {
	ID                                int64
	Code, Name, NativeName, Direction string
	Enabled                           bool
	SortOrder                         int
}
type Profile struct {
	Avatar                                                             string
	Name, Title, Intro, About, Email, Phone, Location, MetaDescription string
}
type Entry struct {
	ID                                                                            int64
	Kind, Image, URL, SecondaryURL, StartDate, EndDate, Proficiency, Technologies string
	Current, Enabled                                                              bool
	SortOrder                                                                     int
	Title, Subtitle, Location, Description, Extra                                 string
}
type Social struct {
	ID               int64
	Label, URL, Icon string
	Enabled          bool
	SortOrder        int
}
type Resume struct {
	Language                              Language
	Profile                               Profile
	Entries                               map[string][]Entry
	Socials                               []Social
	WebsiteTemplate, PDFTemplate, BaseURL string
}
type Dashboard struct {
	Languages, Experiences, Education, Projects, Skills int
	WebsiteTemplate, PDFTemplate                        string
}
