package client

// LiteMarket is a summary representation of a Manifold market.
type LiteMarket struct {
	ID                string   `json:"id"`
	CreatorID         string   `json:"creatorId"`
	CreatorUsername   string   `json:"creatorUsername"`
	CreatorName       string   `json:"creatorName"`
	CreatedTime       int64    `json:"createdTime"`
	CloseTime         *int64   `json:"closeTime,omitempty"`
	Question          string   `json:"question"`
	URL               string   `json:"url"`
	Pool              any      `json:"pool,omitempty"`
	Probability       *float64 `json:"probability,omitempty"`
	P                 *float64 `json:"p,omitempty"`
	TotalLiquidity    *float64 `json:"totalLiquidity,omitempty"`
	OutcomeType       string   `json:"outcomeType"`
	Mechanism         string   `json:"mechanism"`
	Volume            float64  `json:"volume"`
	Volume24Hours     float64  `json:"volume24Hours"`
	IsResolved        bool     `json:"isResolved"`
	Resolution        *string  `json:"resolution,omitempty"`
	ResolutionTime    *int64   `json:"resolutionTime,omitempty"`
	LastBetTime       *int64   `json:"lastBetTime,omitempty"`
	LastCommentTime   *int64   `json:"lastCommentTime,omitempty"`
	LastUpdatedTime   *int64   `json:"lastUpdatedTime,omitempty"`
	UniqueSettorCount *int     `json:"uniqueBettorCount,omitempty"`
	Min               *float64 `json:"min,omitempty"`
	Max               *float64 `json:"max,omitempty"`
	IsLogScale        *bool    `json:"isLogScale,omitempty"`
	GroupSlugs        []string `json:"groupSlugs,omitempty"`
	LiteMarketExtraFields
}

// LiteMarketExtraFields holds fields that may appear in lite market responses.
type LiteMarketExtraFields struct {
	Token             string  `json:"token,omitempty"`
	SiblingContractID *string `json:"siblingContractId,omitempty"`
}

// FullMarket embeds LiteMarket and adds detailed fields.
type FullMarket struct {
	LiteMarket
	Answers         []Answer `json:"answers,omitempty"`
	Description     any      `json:"description,omitempty"`
	TextDescription string   `json:"textDescription,omitempty"`
	CoverImageURL   *string  `json:"coverImageUrl,omitempty"`
}

// Answer represents a possible answer in a multiple choice market.
type Answer struct {
	ID             string   `json:"id"`
	Index          *int     `json:"index,omitempty"`
	ContractID     string   `json:"contractId"`
	CreatedTime    int64    `json:"createdTime"`
	UserID         string   `json:"userId"`
	Text           string   `json:"text"`
	Probability    *float64 `json:"probability,omitempty"`
	Pool           any      `json:"pool,omitempty"`
	Resolution     *string  `json:"resolution,omitempty"`
	ResolutionTime *int64   `json:"resolutionTime,omitempty"`
}

// User represents a Manifold user profile.
type User struct {
	ID            string        `json:"id"`
	CreatedTime   int64         `json:"createdTime"`
	Name          string        `json:"name"`
	Username      string        `json:"username"`
	URL           string        `json:"url"`
	AvatarURL     *string       `json:"avatarUrl,omitempty"`
	Bio           *string       `json:"bio,omitempty"`
	Balance       float64       `json:"balance"`
	TotalDeposits float64       `json:"totalDeposits"`
	ProfitCached  *ProfitCached `json:"profitCached,omitempty"`
}

// ProfitCached holds cached profit information.
type ProfitCached struct {
	Daily   float64 `json:"daily"`
	Weekly  float64 `json:"weekly"`
	Monthly float64 `json:"monthly"`
	AllTime float64 `json:"allTime"`
}

// Bet represents a bet on a market.
type Bet struct {
	ID          string   `json:"id"`
	UserID      string   `json:"userId"`
	ContractID  string   `json:"contractId"`
	CreatedTime int64    `json:"createdTime"`
	Amount      float64  `json:"amount"`
	Outcome     string   `json:"outcome"`
	Shares      float64  `json:"shares"`
	ProbBefore  float64  `json:"probBefore"`
	ProbAfter   float64  `json:"probAfter"`
	IsFilled    *bool    `json:"isFilled,omitempty"`
	IsCancelled *bool    `json:"isCancelled,omitempty"`
	OrderAmount *float64 `json:"orderAmount,omitempty"`
	LimitProb   *float64 `json:"limitProb,omitempty"`
	Fees        *Fees    `json:"fees,omitempty"`
	Fills       []Fill   `json:"fills,omitempty"`
	AnswerID    *string  `json:"answerId,omitempty"`
}

// Fill represents a fill of a limit order.
type Fill struct {
	MatchedBetID *string `json:"matchedBetId,omitempty"`
	Amount       float64 `json:"amount"`
	Shares       float64 `json:"shares"`
	Timestamp    int64   `json:"timestamp"`
}

// Fees represents the fee breakdown on a bet.
type Fees struct {
	CreatorFee   float64 `json:"creatorFee"`
	PlatformFee  float64 `json:"platformFee"`
	LiquidityFee float64 `json:"liquidityFee"`
}

// Comment represents a comment on a market.
type Comment struct {
	ID           string `json:"id"`
	ContractID   string `json:"contractId"`
	UserID       string `json:"userId"`
	UserName     string `json:"userName"`
	UserUsername string `json:"userUsername"`
	CreatedTime  int64  `json:"createdTime"`
	Content      any    `json:"content,omitempty"`
	Markdown     string `json:"markdown,omitempty"`
}

// ContractMetric represents a user's position in a market.
type ContractMetric struct {
	ContractID   string  `json:"contractId"`
	UserID       string  `json:"userId"`
	UserName     string  `json:"userName"`
	UserUsername string  `json:"userUsername"`
	HasShares    bool    `json:"hasShares"`
	TotalShares  any     `json:"totalShares,omitempty"`
	Profit       float64 `json:"profit"`
	HasNoShares  bool    `json:"hasNoShares"`
	HasYesShares bool    `json:"hasYesShares"`
	AnswerID     *string `json:"answerId,omitempty"`
}

// PlaceBetRequest is the request body for placing a bet.
type PlaceBetRequest struct {
	Amount     float64  `json:"amount"`
	ContractID string   `json:"contractId"`
	Outcome    string   `json:"outcome,omitempty"`
	LimitProb  *float64 `json:"limitProb,omitempty"`
	ExpiresAt  *int64   `json:"expiresAt,omitempty"`
	DryRun     *bool    `json:"dryRun,omitempty"`
	AnswerID   string   `json:"answerId,omitempty"`
}

// SellSharesRequest is the request body for selling shares.
type SellSharesRequest struct {
	Outcome  string   `json:"outcome,omitempty"`
	Shares   *float64 `json:"shares,omitempty"`
	AnswerID string   `json:"answerId,omitempty"`
}

// CreateMarketRequest is the request body for creating a market.
type CreateMarketRequest struct {
	OutcomeType string   `json:"outcomeType"`
	Question    string   `json:"question"`
	Description string   `json:"description,omitempty"`
	CloseTime   *int64   `json:"closeTime,omitempty"`
	InitialProb *float64 `json:"initialProb,omitempty"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
	IsLogScale  *bool    `json:"isLogScale,omitempty"`
	Answers     []string `json:"answers,omitempty"`
}

// ResolveMarketRequest is the request body for resolving a market.
type ResolveMarketRequest struct {
	Outcome        string   `json:"outcome"`
	Value          *float64 `json:"value,omitempty"`
	ProbabilityInt *int     `json:"probabilityInt,omitempty"`
	AnswerID       string   `json:"answerId,omitempty"`
}

// CloseMarketRequest is the request body for closing a market.
type CloseMarketRequest struct {
	CloseTime *int64 `json:"closeTime,omitempty"`
}

// AddCommentRequest is the request body for adding a comment.
type AddCommentRequest struct {
	ContractID string `json:"contractId"`
	Markdown   string `json:"markdown"`
}

// AddLiquidityRequest is the request body for adding liquidity.
type AddLiquidityRequest struct {
	Amount float64 `json:"amount"`
}

// SendManaRequest is the request body for sending mana.
type SendManaRequest struct {
	ToIDs   []string `json:"toIds"`
	Amount  float64  `json:"amount"`
	Message string   `json:"message,omitempty"`
}
