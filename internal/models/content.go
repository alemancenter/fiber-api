package models

import "time"

// SchoolClass represents a grade level
type SchoolClass struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"type:varchar(255);not null" json:"name"`
	GradeLevel string    `gorm:"type:varchar(50)" json:"grade_level"`
	CountryID  *uint     `json:"country_id,omitempty"`
	IsActive   bool      `gorm:"default:true" json:"is_active"`
	Order      int       `gorm:"default:0" json:"order"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	Subjects  []Subject  `gorm:"foreignKey:SchoolClassID" json:"subjects,omitempty"`
	Semesters []Semester `gorm:"foreignKey:SchoolClassID" json:"semesters,omitempty"`
}

func (SchoolClass) TableName() string { return "school_classes" }

// Subject represents a school subject (Math, English, etc.)
type Subject struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`
	SchoolClassID *uint   `gorm:"index" json:"school_class_id,omitempty"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	Order       int       `gorm:"default:0" json:"order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	SchoolClass *SchoolClass `gorm:"foreignKey:SchoolClassID" json:"school_class,omitempty"`
	Semesters   []Semester   `gorm:"foreignKey:SubjectID" json:"semesters,omitempty"`
}

func (Subject) TableName() string { return "subjects" }

// Semester represents an academic term
type Semester struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`
	SubjectID   *uint     `gorm:"index" json:"subject_id,omitempty"`
	SchoolClassID *uint   `gorm:"index" json:"school_class_id,omitempty"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	Order       int       `gorm:"default:0" json:"order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	Subject     *Subject     `gorm:"foreignKey:SubjectID" json:"subject,omitempty"`
	SchoolClass *SchoolClass `gorm:"foreignKey:SchoolClassID" json:"school_class,omitempty"`
}

func (Semester) TableName() string { return "semesters" }

// Category represents a post category
type Category struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`
	Slug        string    `gorm:"type:varchar(300);unique" json:"slug"`
	Description *string   `gorm:"type:text" json:"description,omitempty"`
	Image       *string   `gorm:"type:varchar(500)" json:"image,omitempty"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	Order       int       `gorm:"default:0" json:"order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	Posts []Post `gorm:"foreignKey:CategoryID" json:"posts,omitempty"`
}

func (Category) TableName() string { return "categories" }

// Keyword represents a tag/keyword used in articles and posts
type Keyword struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"type:varchar(255);unique;not null" json:"name"`
	Slug      string    `gorm:"type:varchar(300);unique" json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Keyword) TableName() string { return "keywords" }

// File represents an attached document/file
type File struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ArticleID    *uint     `gorm:"index" json:"article_id,omitempty"`
	PostID       *uint     `gorm:"index" json:"post_id,omitempty"`
	FilePath     string    `gorm:"type:varchar(500);not null" json:"file_path"`
	FileType     string    `gorm:"type:varchar(50)" json:"file_type"`
	FileCategory *string   `gorm:"type:varchar(100)" json:"file_category,omitempty"`
	FileName     string    `gorm:"type:varchar(255)" json:"file_name"`
	FileSize     int64     `json:"file_size"`
	MimeType     string    `gorm:"type:varchar(100)" json:"mime_type"`
	ViewCount    int       `gorm:"default:0" json:"view_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	Article *Article `gorm:"foreignKey:ArticleID" json:"article,omitempty"`
	Post    *Post    `gorm:"foreignKey:PostID" json:"post,omitempty"`
}

func (File) TableName() string { return "files" }

// Comment represents a polymorphic comment
type Comment struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Body            string    `gorm:"type:text;not null" json:"body"`
	UserID          uint      `gorm:"not null" json:"user_id"`
	CommentableID   uint      `gorm:"not null" json:"commentable_id"`
	CommentableType string    `gorm:"type:varchar(255);not null" json:"commentable_type"`
	Database        string    `gorm:"type:varchar(10);default:'jo'" json:"database"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	User      *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Reactions []Reaction `gorm:"foreignKey:CommentID" json:"reactions,omitempty"`
}

func (Comment) TableName() string { return "comments" }

// Reaction represents an emoji reaction on a comment
type Reaction struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CommentID uint      `gorm:"not null;index" json:"comment_id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Emoji     string    `gorm:"type:varchar(20)" json:"emoji"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	User    *User    `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Comment *Comment `gorm:"foreignKey:CommentID" json:"comment,omitempty"`
}

func (Reaction) TableName() string { return "reactions" }

// Event represents a calendar event
type Event struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Title       string     `gorm:"type:varchar(500);not null" json:"title"`
	Description *string    `gorm:"type:text" json:"description,omitempty"`
	StartDate   time.Time  `json:"start_date"`
	EndDate     *time.Time `json:"end_date,omitempty"`
	AllDay      bool       `gorm:"default:false" json:"all_day"`
	Color       *string    `gorm:"type:varchar(50)" json:"color,omitempty"`
	Database    string     `gorm:"type:varchar(10);default:'jo'" json:"database"`
	CreatedBy   *uint      `json:"created_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (Event) TableName() string { return "events" }

// Setting represents an application setting key-value pair
type Setting struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"type:varchar(255);unique;not null" json:"key"`
	Value     *string   `gorm:"type:longtext" json:"value,omitempty"`
	Group     string    `gorm:"type:varchar(100);default:'general'" json:"group"`
	Type      string    `gorm:"type:varchar(50);default:'string'" json:"type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Setting) TableName() string { return "settings" }

// Notification represents a user notification
type Notification struct {
	ID        string     `gorm:"type:char(36);primaryKey" json:"id"`
	Type      string     `gorm:"type:varchar(255);not null" json:"type"`
	NotifiableType string `gorm:"type:varchar(255);not null" json:"-"`
	NotifiableID   uint   `gorm:"not null" json:"notifiable_id"`
	Data      string     `gorm:"type:json" json:"data"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (Notification) TableName() string { return "notifications" }

// Message represents a direct message between users
type Message struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	SenderID    uint       `gorm:"not null;index" json:"sender_id"`
	RecipientID uint       `gorm:"not null;index" json:"recipient_id"`
	Subject     *string    `gorm:"type:varchar(500)" json:"subject,omitempty"`
	Body        string     `gorm:"type:text;not null" json:"body"`
	IsRead      bool       `gorm:"default:false" json:"is_read"`
	IsImportant bool       `gorm:"default:false" json:"is_important"`
	IsDraft     bool       `gorm:"default:false" json:"is_draft"`
	ReadAt      *time.Time `json:"read_at,omitempty"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	Sender    *User `gorm:"foreignKey:SenderID" json:"sender,omitempty"`
	Recipient *User `gorm:"foreignKey:RecipientID" json:"recipient,omitempty"`
}

func (Message) TableName() string { return "messages" }
