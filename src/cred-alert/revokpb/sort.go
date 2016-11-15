package revokpb

type RCCByName []*RepositoryCredentialCount

func (a RCCByName) Len() int           { return len(a) }
func (a RCCByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a RCCByName) Less(i, j int) bool { return a[i].Name < a[j].Name }

type BCCByName []*BranchCredentialCount

func (a BCCByName) Len() int           { return len(a) }
func (a BCCByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a BCCByName) Less(i, j int) bool { return a[i].Name < a[j].Name }
