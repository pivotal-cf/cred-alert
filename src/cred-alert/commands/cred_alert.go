package commands

type CredAlertCommand struct {
	Scan ScanCommand `command:"scan" description:"Scan an archive, Git diff, or input from STDIN"`
}

var CredAlert CredAlertCommand
