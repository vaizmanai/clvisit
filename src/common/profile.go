package common

type Profile struct {
	Email string
	Pass  string

	Contacts *Contact
}

type Contact struct {
	Id      int
	Caption string
	Type    string //node - контакт, fold - папка
	Pid     string

	Inner *Contact
	Next  *Contact
}
