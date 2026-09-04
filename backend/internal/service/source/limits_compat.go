package source

import domainsource "github.com/Benzocloud/cosmohack/backend/internal/domain/source"

type Limits = domainsource.Limits
type LimitsSpec = domainsource.LimitsSpec

var DefaultLimits = domainsource.DefaultLimits
var NewLimits = domainsource.NewLimits

const limitsProvider = "collector"
