package client

import "time"

// Resource wraps a WaniKani API resource with its metadata envelope.
type Resource[T any] struct {
	ID            int       `json:"id"`
	Object        string    `json:"object"`
	URL           string    `json:"url"`
	DataUpdatedAt time.Time `json:"data_updated_at"`
	Data          T         `json:"data"`
}

// Collection is the paginated collection response from the WaniKani API.
type Collection[T any] struct {
	Object        string        `json:"object"`
	URL           string        `json:"url"`
	DataUpdatedAt time.Time     `json:"data_updated_at"`
	Pages         Pages         `json:"pages"`
	TotalCount    int           `json:"total_count"`
	Data          []Resource[T] `json:"data"`
}

// Pages contains pagination info for a collection.
type Pages struct {
	NextURL     string `json:"next_url"`
	PreviousURL string `json:"previous_url"`
	PerPage     int    `json:"per_page"`
}

// User represents a WaniKani user profile.
type User struct {
	ID                       string       `json:"id"`
	Username                 string       `json:"username"`
	Level                    int          `json:"level"`
	ProfileURL               string       `json:"profile_url"`
	StartedAt                time.Time    `json:"started_at"`
	CurrentVacationStartedAt *time.Time   `json:"current_vacation_started_at"`
	Subscription             Subscription `json:"subscription"`
}

// Subscription describes a user's WaniKani subscription.
type Subscription struct {
	Active          bool       `json:"active"`
	Type            string     `json:"type"`
	MaxLevelGranted int        `json:"max_level_granted"`
	PeriodEndsAt    *time.Time `json:"period_ends_at"`
}

// Summary contains upcoming lesson and review availability.
type Summary struct {
	Lessons []SummaryEntry `json:"lessons"`
	Reviews []SummaryEntry `json:"reviews"`
}

// SummaryEntry is a time slot with available subject IDs.
type SummaryEntry struct {
	AvailableAt time.Time `json:"available_at"`
	SubjectIDs  []int     `json:"subject_ids"`
}

// Assignment tracks a user's progress on a specific subject.
type Assignment struct {
	AvailableAt   *time.Time `json:"available_at"`
	BurnedAt      *time.Time `json:"burned_at"`
	CreatedAt     time.Time  `json:"created_at"`
	Level         int        `json:"level"`
	PassedAt      *time.Time `json:"passed_at"`
	ResurrectedAt *time.Time `json:"resurrected_at"`
	SRSStage      int        `json:"srs_stage"`
	SRSStageName  string     `json:"srs_stage_name"`
	StartedAt     *time.Time `json:"started_at"`
	SubjectID     int        `json:"subject_id"`
	SubjectType   string     `json:"subject_type"`
	UnlockedAt    *time.Time `json:"unlocked_at"`
}

// Subject represents a kanji, vocabulary, or radical.
type Subject struct {
	Characters             *string   `json:"characters"`
	CreatedAt              time.Time `json:"created_at"`
	DocumentURL            string    `json:"document_url"`
	Level                  int       `json:"level"`
	Meanings               []Meaning `json:"meanings"`
	Readings               []Reading `json:"readings,omitempty"`
	Slug                   string    `json:"slug"`
	MeaningMnemonic        string    `json:"meaning_mnemonic"`
	ReadingMnemonic        string    `json:"reading_mnemonic,omitempty"`
	ComponentSubjectIDs    []int     `json:"component_subject_ids,omitempty"`
	AmalgamationSubjectIDs []int     `json:"amalgamation_subject_ids,omitempty"`
}

// Meaning is a meaning entry for a subject.
type Meaning struct {
	Meaning        string `json:"meaning"`
	Primary        bool   `json:"primary"`
	AcceptedAnswer bool   `json:"accepted_answer"`
}

// Reading is a reading entry for a subject (kanji/vocab).
type Reading struct {
	Reading        string `json:"reading"`
	Primary        bool   `json:"primary"`
	AcceptedAnswer bool   `json:"accepted_answer"`
	Type           string `json:"type,omitempty"`
}

// ReviewStatistic tracks review accuracy for a subject.
type ReviewStatistic struct {
	CreatedAt            time.Time `json:"created_at"`
	MeaningCorrect       int       `json:"meaning_correct"`
	MeaningIncorrect     int       `json:"meaning_incorrect"`
	MeaningMaxStreak     int       `json:"meaning_max_streak"`
	MeaningCurrentStreak int       `json:"meaning_current_streak"`
	ReadingCorrect       int       `json:"reading_correct"`
	ReadingIncorrect     int       `json:"reading_incorrect"`
	ReadingMaxStreak     int       `json:"reading_max_streak"`
	ReadingCurrentStreak int       `json:"reading_current_streak"`
	PercentageCorrect    int       `json:"percentage_correct"`
	SubjectID            int       `json:"subject_id"`
	SubjectType          string    `json:"subject_type"`
}

// LevelProgression tracks progress through a WaniKani level.
type LevelProgression struct {
	AbandonedAt *time.Time `json:"abandoned_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time  `json:"created_at"`
	Level       int        `json:"level"`
	PassedAt    *time.Time `json:"passed_at"`
	StartedAt   *time.Time `json:"started_at"`
	UnlockedAt  *time.Time `json:"unlocked_at"`
}
