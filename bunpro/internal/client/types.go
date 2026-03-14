package client

// UserResponse is the top-level response from GET /user.
type UserResponse struct {
	User            ResourceEnvelope[UserAttributes] `json:"user"`
	ActiveCosmetics CollectionEnvelope[CosmeticItem] `json:"active_cosmetics"`
	ActiveTitle     *string                          `json:"active_title"`
}

// ResourceEnvelope wraps a single JSON:API resource.
type ResourceEnvelope[T any] struct {
	Data Resource[T] `json:"data"`
}

// CollectionEnvelope wraps a JSON:API collection.
type CollectionEnvelope[T any] struct {
	Data []Resource[T] `json:"data"`
}

// Resource is a JSON:API resource with typed attributes.
type Resource[T any] struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes T      `json:"attributes"`
}

// UserAttributes contains user profile fields.
type UserAttributes struct {
	ID                    int     `json:"id"`
	Username              string  `json:"username"`
	Level                 int     `json:"level"`
	XP                    int     `json:"xp"`
	NextLevelXP           int     `json:"next_level_xp"`
	PrevLevelXP           int     `json:"prev_level_xp"`
	Language              string  `json:"language"`
	VacationMode          bool    `json:"vacation_mode"`
	Furigana              string  `json:"furigana"`
	HasActiveSubscription bool    `json:"has_active_subscription"`
	IsLifetime            bool    `json:"is_lifetime"`
	CreatedAt             string  `json:"created_at"`
	UpdatedAt             string  `json:"updated_at"`
	StartPage             string  `json:"start_page"`
	AvatarURL             string  `json:"avatar_url"`
	LightMode             string  `json:"light_mode"`
	HideStreak            string  `json:"hide_streak"`
	BunCoin               int     `json:"buncoin"`
	FavoriteBadge         *string `json:"favorite_badge"`
}

// CosmeticItem represents an active cosmetic.
type CosmeticItem struct {
	ID           int    `json:"id"`
	CosmeticType string `json:"cosmetic_type"`
	Category     string `json:"category"`
	Title        string `json:"title"`
	Description  string `json:"description"`
}

// DueCount is the response from GET /user/due.
type DueCount struct {
	TotalDueGrammar int `json:"total_due_grammar"`
	TotalDueVocab   int `json:"total_due_vocab"`
}

// DeckSetting represents a study deck configuration from GET /user/queue.
type DeckSetting struct {
	ID                    int    `json:"id"`
	DeckID                int    `json:"deck_id"`
	BatchSize             int    `json:"batch_size"`
	DailyGoal             int    `json:"daily_goal"`
	DailyGoalCountGrammar int    `json:"daily_goal_count_grammar"`
	DailyGoalCountVocab   int    `json:"daily_goal_count_vocab"`
	CompleteGrammarCount  int    `json:"complete_grammar_count"`
	CompleteVocabCount    int    `json:"complete_vocab_count"`
	LastLearnedAt         string `json:"last_learned_at"`
	SortingOrder          string `json:"sorting_order"`
	IsBookmarked          bool   `json:"is_bookmarked"`
}

// BaseStats is the response from GET /user_stats/base_stats.
type BaseStats struct {
	Facts  Facts                         `json:"facts"`
	Badges CollectionEnvelope[BadgeAttr] `json:"badges"`
}

// Facts are the core user statistics.
type Facts struct {
	DaysStudied    int           `json:"days_studied"`
	Streak         int           `json:"streak"`
	LastSession    float64       `json:"last_session"`
	GrammarStudied int           `json:"grammar_studied"`
	VocabStudied   int           `json:"vocab_studied"`
	TotalBadges    int           `json:"total_badges"`
	WeeklyStreak   []StreakEntry `json:"weekly_streak"`
}

// StreakEntry is a single day in the weekly streak.
type StreakEntry struct {
	Day string `json:"day"`
	Val bool   `json:"val"`
}

// BadgeAttr represents a badge.
type BadgeAttr struct {
	ID                   int     `json:"id"`
	BadgeImage           string  `json:"badge_image"`
	PercentOfUsersEarned float64 `json:"percent_of_users_earned"`
	Category             string  `json:"category"`
	Rarity               string  `json:"rarity"`
	Title                string  `json:"title"`
	FlavorText           string  `json:"flavor_text"`
	HumanRequirement     string  `json:"human_requirement"`
}

// GrammarVocabMap is a map from date/time key to count, split by grammar and vocab.
// Used for forecast_daily, forecast_hourly, review_activity.
type GrammarVocabMap struct {
	Grammar map[string]int `json:"grammar"`
	Vocab   map[string]int `json:"vocab"`
}

// JLPTProgress is the response from GET /user_stats/jlpt_progress_mixed.
type JLPTProgress struct {
	Grammar map[string]SRSLevelCount `json:"grammar"`
	Vocab   map[string]SRSLevelCount `json:"vocab"`
}

// SRSLevelCount has counts per SRS stage.
type SRSLevelCount struct {
	Beginner   int `json:"beginner"`
	Adept      int `json:"adept"`
	Seasoned   int `json:"seasoned"`
	Expert     int `json:"expert"`
	Master     int `json:"master"`
	TotalCount int `json:"total_count"`
}

// SRSOverview is the response from GET /user_stats/srs_level_overview.
type SRSOverview struct {
	Grammar SRSOverviewCounts `json:"grammar"`
	Vocab   SRSOverviewCounts `json:"vocab"`
}

// SRSOverviewCounts includes ghost/self_study in addition to SRS stages.
type SRSOverviewCounts struct {
	Beginner  int `json:"beginner"`
	Adept     int `json:"adept"`
	Seasoned  int `json:"seasoned"`
	Expert    int `json:"expert"`
	Master    int `json:"master"`
	Ghost     int `json:"ghost"`
	SelfStudy int `json:"self_study"`
}

// SRSLevelDetailsResponse is the response from GET /user_stats/srs_level_details.
type SRSLevelDetailsResponse struct {
	Type    string                     `json:"type"`
	Reviews CollectionEnvelope[Review] `json:"reviews"`
}

// Review represents a single review item.
type Review struct {
	ID                int    `json:"id"`
	Streak            int    `json:"streak"`
	NextReview        string `json:"next_review"`
	Complete          bool   `json:"complete"`
	Accuracy          int    `json:"accuracy"`
	TimesStudied      int    `json:"times_studied"`
	ReviewableID      int    `json:"reviewable_id"`
	ReviewableType    string `json:"reviewable_type"`
	StartedStudyingAt string `json:"started_studying_at"`
	GhostCount        int    `json:"ghost_count"`
}

// GrammarPointResponse wraps a grammar point resource.
type GrammarPointResponse struct {
	Data Resource[GrammarPointAttributes] `json:"data"`
}

// GrammarPointAttributes contains grammar point fields.
type GrammarPointAttributes struct {
	ID                      int    `json:"id"`
	Title                   string `json:"title"`
	Meaning                 string `json:"meaning"`
	Slug                    string `json:"slug"`
	Level                   string `json:"level"`
	LessonID                int    `json:"lesson_id"`
	PartOfSpeech            string `json:"part_of_speech"`
	Register                string `json:"register"`
	WordType                string `json:"word_type"`
	Nuance                  string `json:"nuance"`
	NuanceTranslation       string `json:"nuance_translation"`
	CasualStructure         string `json:"casual_structure"`
	PoliteStructure         string `json:"polite_structure"`
	Caution                 string `json:"caution"`
	GrammarOrder            int    `json:"grammar_order"`
	PartOfSpeechTranslation string `json:"part_of_speech_translation"`
	RegisterTranslation     string `json:"register_translation"`
	Metadata                string `json:"metadata"`
	DiscourseLink           string `json:"discourse_link"`
}

// VocabResponse wraps a vocab resource.
type VocabResponse struct {
	Data Resource[VocabAttributes] `json:"data"`
}

// VocabAttributes contains vocabulary fields.
type VocabAttributes struct {
	ID                int         `json:"id"`
	Title             string      `json:"title"`
	JLPTLevel         string      `json:"jlpt_level"`
	Slug              string      `json:"slug"`
	Furigana          string      `json:"furigana"`
	Kana              string      `json:"kana"`
	PitchAccentStress string      `json:"pitch_accent_stress"`
	FrequencyAnime    int         `json:"frequency_anime"`
	FrequencyNovels   int         `json:"frequency_novels"`
	FrequencyGeneral  int         `json:"frequency_general"`
	FrequencyNetflix  int         `json:"frequency_netflix"`
	HasTTSAudio       bool        `json:"has_tts_audio"`
	JMDictData        *JMDictData `json:"jmdict_data"`
}

// JMDictData contains dictionary data for a vocab entry.
type JMDictData struct {
	ID    string        `json:"id"`
	Kanji []JMDictForm  `json:"kanji"`
	Kana  []JMDictForm  `json:"kana"`
	Sense []JMDictSense `json:"sense"`
}

// JMDictForm is a kanji or kana reading form.
type JMDictForm struct {
	Text   string `json:"text"`
	Common bool   `json:"common"`
}

// JMDictSense is a meaning/sense entry.
type JMDictSense struct {
	PartOfSpeech []string      `json:"partOfSpeech"`
	Gloss        []JMDictGloss `json:"gloss"`
}

// JMDictGloss is a single gloss/translation.
type JMDictGloss struct {
	Lang string `json:"lang"`
	Text string `json:"text"`
}
