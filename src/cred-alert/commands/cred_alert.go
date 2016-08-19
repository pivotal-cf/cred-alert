package commands

type CredAlertCommand struct {
	Scan   ScanCommand   `command:"scan" description:"Scan an archive, Git diff, or input from STDIN"`
	Update UpdateCommand `command:"update" description:"Update cred-alert to the latest version"`
}

var CredAlert CredAlertCommand
