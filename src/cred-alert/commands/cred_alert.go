package commands

type CredAlertCommand struct {
	Scan    ScanCommand    `command:"scan" description:"Scan an archive, Git diff, or input from STDIN"`
	Update  UpdateCommand  `command:"update" description:"Update cred-alert to the latest version"`
	Version VersionCommand `command:"version" description:"Displays cred-alert version" alias:"V"`
}

var CredAlert CredAlertCommand
