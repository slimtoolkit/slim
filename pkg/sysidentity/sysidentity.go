package sysidentity

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	PasswdFilePath    = "/etc/passwd"
	ShadowFilePath    = "/etc/shadow"
	GshadowFilePath   = "/etc/gshadow"
	GroupFilePath     = "/etc/group"
	LoginDefsFilePath = "/etc/login.defs"
	SudoersFilePath   = "/etc/sudoers"
	AuthKeysFileName  = "authorized_keys"

	//todo: move to one of the runtime detection packages
	AuthLogFilePath = "/var/log/auth.log"
)

func IsSourceFile(fullPath string) bool {
	if IsAuthKeyFile(fullPath) {
		return true
	}

	switch fullPath {
	case PasswdFilePath, ShadowFilePath, GroupFilePath:
		return true
	}

	return false
}

func IsAuthKeyFile(fullPath string) bool {
	if filepath.Base(fullPath) == AuthKeysFileName {
		return true
	}

	return false
}

func NewDataSet() *DataSet {
	ref := &DataSet{
		AuthKeysData: map[string][]byte{},
	}

	return ref
}

type DataSet struct {
	PasswdFilePath string
	PasswdData     []byte
	ShadowFilePath string
	ShadowData     []byte
	GroupFilePath  string
	GroupData      []byte
	AuthKeysData   map[string][]byte
}

func (ref *DataSet) AddData(filePath string, data []byte) bool {
	switch filePath {
	case PasswdFilePath:
		ref.PasswdFilePath = filePath
		ref.PasswdData = data
		return true
	case ShadowFilePath:
		ref.ShadowFilePath = filePath
		ref.ShadowData = data
		return true
	case GroupFilePath:
		ref.GroupFilePath = filePath
		ref.GroupData = data
		return true
	default:
		if IsAuthKeyFile(filePath) {
			if ref.AuthKeysData == nil {
				ref.AuthKeysData = map[string][]byte{}
			}

			ref.AuthKeysData[filePath] = data
			return true
		}
	}

	return false
}

func NewReportFromData(data *DataSet) (*Report, error) {
	pinfo, err := ReadPasswdData(data.PasswdData)
	if err != nil {
		return nil, err
	}

	sinfo, err := ReadShadowData(data.ShadowData)
	if err != nil {
		return nil, err
	}

	ginfo, err := ReadGroupData(data.GroupData)
	if err != nil {
		return nil, err
	}

	report := Report{
		Sources: []*DataSource{
			{FilePath: data.PasswdFilePath},
			{FilePath: data.ShadowFilePath},
			{FilePath: data.GroupFilePath},
		},
		Users:  map[string]*UserInfo{},
		Groups: map[string]*GroupInfo{},
	}

	for _, record := range pinfo.Records {
		userInfo := &UserInfo{
			Username:       record.Username,
			PasswdPassword: record.Password,
			UID:            record.UID,
			GID:            record.GID,
			ExtraInfo:      record.Info,
			Home:           record.Home,
			Shell:          record.Shell,
			NoLoginShell:   record.NoLoginShell,
		}

		report.Users[userInfo.Username] = userInfo
	}

	for _, record := range sinfo.Records {
		userInfo, found := report.Users[record.Username]
		if !found {
			userInfo := &UserInfo{
				Username: record.Username,
			}

			report.Users[record.Username] = userInfo
		}

		userInfo.ShadowPassword = record.Password
		userInfo.LastChangeRaw = record.LastChangeRaw
		userInfo.LastChangeDate = record.LastChangeDate
		userInfo.MinimumAge = record.MinimumAge
		userInfo.MaximumAge = record.MaximumAge
		userInfo.WarningPeriod = record.WarningPeriod
		userInfo.InactiveDays = record.InactiveDays
		userInfo.ExpirationRaw = record.ExpirationRaw
		userInfo.ExpirationDate = record.ExpirationDate

	}

	for _, record := range ginfo.Records {
		groupInfo := &GroupInfo{
			Name:     record.Group,
			ID:       record.GID,
			Members:  record.Members,
			Password: record.Password, //todo: get the actual password infor from gshadow (if any)
		}

		report.Groups[groupInfo.Name] = groupInfo
	}

	for fileName, data := range data.AuthKeysData {
		//tmp
		fmt.Printf("%s -> %v\n", fileName, data)
	}

	return &report, nil
}

type Report struct {
	Users   map[string]*UserInfo  `json:"users"`
	Groups  map[string]*GroupInfo `json:"groups"`
	Sources []*DataSource         `json:"sources"`
}

type DataSource struct {
	FilePath string             `json:"file_path"`
	Metadata DataSourceMetadata `json:"metadata"`
}

type DataSourceMetadata struct {
	Sha1Hash string `json:"sha1_hash,omitempty"`
	FileSize int64  `json:"file_size"`
	ModeText string `json:"mode"`
	//ModTime time.Time `json:"mod_time,omitempty"`
	//ChangeTime time.Time `json:"change_time,omitempty"`
	//CreateTime time.Time `json:"create_time,omitempty"`
}

func (ref *Report) StringJSONPretty() string {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent(" ", " ")
	_ = encoder.Encode(ref)
	return out.String()
}

type GroupInfo struct {
	Name     string   `json:"name"`
	ID       int      `json:"id"`
	Members  []string `json:"members"`
	Password string   `json:"password,omitempty"`
}

type UserInfo struct {
	Username       string       `json:"username"`
	PasswdPassword string       `json:"passwd_password"`
	ShadowPassword PasswordHash `json:"shadow_password"`
	UID            int          `json:"uid"`
	GID            int          `json:"gid"`
	ExtraInfo      string       `json:"extra_info"`
	Home           string       `json:"home"`
	Shell          string       `json:"shell"`
	NoLoginShell   bool         `json:"no_login_shell"`

	LastChangeRaw  int       `json:"last_change_raw"`
	LastChangeDate time.Time `json:"last_change_date"`
	MinimumAge     int       `json:"minimum_age"`
	MaximumAge     int       `json:"maximum_age"`
	WarningPeriod  int       `json:"warning_period"`
	InactiveDays   int       `json:"inactive_days"`
	ExpirationRaw  int       `json:"expiration_raw"`
	ExpirationDate time.Time `json:"expiration_date"`

	SshKeys []*SshKeyRecord `json:"ssh_keys,omitempty"`
}

type AuthorizedKeysFileInfo struct {
	Records []SshKeyRecord `json:"records"`
}

type SshKeyRecord struct {
	KeyType      string   `json:"key_type"`
	Key          string   `json:"key"` //base64 encoded
	Comment      string   `json:"comment"`
	Command      string   `json:"command,omitempty"`
	Environments []string `json:"environments,omitempty"`
	OtherOptions []string `json:"other_options,omitempty"`
	RawData      string   `json:"raw_data"`
	FilePath     string   `json:"file_path"`
}

/*
"golang.org/x/crypto/ssh"
func ParseAuthorizedKey(in []byte) (out PublicKey, comment string, options []string, rest []byte, err error)
ssh.ParseAuthorizedKey([]byte(foreverUserCertString))
*/

//Raw data structs

type PasswdFileInfo struct {
	Records []PasswdRecord `json:"records"`
}

type PasswdRecord struct {
	Username     string `json:"username"`
	Password     string `json:"password"` //password hash, "x" if the actual password hash is in the shadow file
	UID          int    `json:"uid"`
	GID          int    `json:"gid"`
	Info         string `json:"info"`  //additional user identity info / GECOS
	Home         string `json:"home"`  //home directory
	Shell        string `json:"shell"` //shell exected when user logs in
	RawData      string `json:"raw_data"`
	NoLoginShell bool   `json:"no_login_shell"`
}

func ReadPasswdFile(filePath string) (*PasswdFileInfo, error) {
	//todo: have an option to skip bad records
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return ReadPasswdData(data)
}

func ReadPasswdData(data []byte) (*PasswdFileInfo, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))

	var info PasswdFileInfo
	for scanner.Scan() {
		record, err := ParsePasswdRecord(scanner.Text())
		if err != nil {
			return nil, err
		}

		info.Records = append(info.Records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &info, nil
}

func ParsePasswdRecord(line string) (PasswdRecord, error) {
	line = strings.TrimSpace(line)
	record := PasswdRecord{
		RawData: line,
	}

	parts := strings.Split(line, ":")
	if len(parts) != 7 {
		return record, errors.New("unexpected field count")
	}

	record.Username = parts[0]
	record.Password = parts[1]
	record.Info = parts[4]
	record.Home = parts[5]
	record.Shell = strings.TrimSpace(parts[6])

	if _, found := NoLoginShells[record.Shell]; found {
		record.NoLoginShell = true
	}

	var err error
	record.UID, err = strconv.Atoi(parts[2])
	if err != nil {
		return record, fmt.Errorf("error parsing UID field - %s (%v)", parts[2], err)
	}

	record.GID, err = strconv.Atoi(parts[3])
	if err != nil {
		return record, fmt.Errorf("error parsing GID field - %s (%v)", parts[3], err)
	}

	return record, nil
}

const (
	HasShadowFileRecord = "x"
)

func (ref PasswdRecord) UsesShadow() bool {
	if ref.Password == HasShadowFileRecord {
		return true
	}

	return false
}

var NoLoginShells = map[string]struct{}{
	"/sbin/nologin":     {},
	"/usr/sbin/nologin": {},
	"/bin/false":        {},
}

const (
	HashTypeDES          = "" //yes, it's empty
	HashTypeMD5          = "1"
	HashTypeBlowfish     = "2a"
	HashTypeBcrypt       = "2b"
	HashTypeEksblowfish  = "2y"
	HashTypeSHA256       = "5"
	HashTypeSHA512       = "6"
	HashTypeYescrypt     = "y"
	HashTypeGostYescrypt = "gy"
	HashTypeScrypt       = "7"
)

var HashTypes = map[string]string{
	HashTypeDES:          "DES",
	HashTypeMD5:          "MD5",
	HashTypeBlowfish:     "blowfish",
	HashTypeBcrypt:       "bcrypt",
	HashTypeEksblowfish:  "eksblowfish",
	HashTypeSHA256:       "SHA256",
	HashTypeSHA512:       "SHA512",
	HashTypeYescrypt:     "yescrypt",
	HashTypeGostYescrypt: "gost-yescrypt",
	HashTypeScrypt:       "scrypt",
}

const (
	NoPasswordLoginUser    = "!"
	NoPasswordLoginService = "*"
)

func NewPasswordHash(data string) PasswordHash {
	var out PasswordHash

	switch data {
	case NoPasswordLoginUser, NoPasswordLoginService:
		out.NoPasswordLogin = true
		return out
	}

	parts := strings.Split(data, "$")

	if len(parts) < 4 {
		return out
	}

	out.AlgoTypeRaw = parts[1]
	out.Salt = parts[len(parts)-2]
	out.Hash = parts[len(parts)-1]

	out.AlgoType = AlgoNameFromType(out.AlgoTypeRaw)

	if len(parts) > 4 {
		out.AlgoParam = parts[2]
	}

	return out
}

func AlgoNameFromType(data string) string {
	name, found := HashTypes[data]
	if found {
		return name
	}

	return "other"
}

type PasswordHash struct {
	AlgoTypeRaw     string `json:"algo_type_raw,omitempty"`
	AlgoType        string `json:"algo_type,omitempty"`
	AlgoParam       string `json:"algo_param,omitempty"` //encoded (need to decode)
	Salt            string `json:"salt,omitempty"`
	Hash            string `json:"hash,omitempty"`
	NoPasswordLogin bool   `json:"no_password_login"`
}

func (ref PasswordHash) UsesWeakAlgo() bool {
	switch ref.AlgoTypeRaw {
	case HashTypeDES, HashTypeMD5:
		return true
	}

	return false
}

func (ref ShadowRecord) LoginWithoutPassword() bool {
	if ref.PasswordRaw == "" {
		return true
	}

	return false
}

type ShadowFileInfo struct {
	Records []ShadowRecord `json:"records"`
}

const FieldNotSet = -1

type ShadowRecord struct {
	Username       string
	PasswordRaw    string
	Password       PasswordHash
	LastChangeRaw  int
	LastChangeDate time.Time
	MinimumAge     int
	MaximumAge     int
	WarningPeriod  int
	InactiveDays   int
	ExpirationRaw  int
	ExpirationDate time.Time
	Reserved       string
	RawData        string
}

func ReadShadowFile(filePath string) (*ShadowFileInfo, error) {
	//todo: have an option to skip bad records
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return ReadShadowData(data)
}

func ReadShadowData(data []byte) (*ShadowFileInfo, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))

	var info ShadowFileInfo
	for scanner.Scan() {
		record, err := ParseShadowRecord(scanner.Text())
		if err != nil {
			return nil, err
		}

		info.Records = append(info.Records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &info, nil
}

func ParseShadowRecord(line string) (ShadowRecord, error) {
	line = strings.TrimSpace(line)
	record := ShadowRecord{
		RawData: line,
	}

	parts := strings.Split(line, ":")
	if len(parts) != 9 {
		return record, errors.New("unexpected field count")
	}

	record.Username = parts[0]
	record.PasswordRaw = parts[1]
	record.Reserved = parts[8]

	intFields := [...]*int{
		&record.LastChangeRaw,
		&record.MinimumAge,
		&record.MaximumAge,
		&record.WarningPeriod,
		&record.InactiveDays,
		&record.ExpirationRaw,
	}

	for idx, val := range intFields {
		field := parts[idx+2]
		if field == "" {
			*val = FieldNotSet
		} else {
			var err error
			*val, err = strconv.Atoi(field)
			if err != nil {
				return record, fmt.Errorf("error parsing field - %s (%v)", field, err)
			}
		}
	}

	record.Password = NewPasswordHash(record.PasswordRaw)
	var err error
	record.LastChangeDate, err = daysToDate(record.LastChangeRaw)
	if err != nil {
		return record, err
	}

	record.ExpirationDate, err = daysToDate(record.ExpirationRaw)
	if err != nil {
		return record, err
	}

	return record, nil
}

func daysToDate(days int) (time.Time, error) {
	if days == FieldNotSet {
		return time.Time{}, nil
	}

	return time.Date(1970, 0, 0, 0, 0, 0, 0, time.UTC).AddDate(0, 0, days), nil
}

////////////

type GroupRecord struct {
	Group      string   `json:"gid"`      //group name
	Password   string   `json:"password"` //password hash, usually empty / unused (actual password hashes are in gshadow)
	GID        int      `json:"gid"`
	MembersRaw string   `json:"members_raw"`
	Members    []string `json:"members"`
	RawData    string   `json:"raw_data"`
}

type GroupFileInfo struct {
	Records []GroupRecord `json:"records"`
}

func ReadGroupFile(filePath string) (*GroupFileInfo, error) {
	//todo: have an option to skip bad records
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return ReadGroupData(data)
}

func ReadGroupData(data []byte) (*GroupFileInfo, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))

	var info GroupFileInfo
	for scanner.Scan() {
		record, err := ParseGroupRecord(scanner.Text())
		if err != nil {
			return nil, err
		}

		info.Records = append(info.Records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &info, nil
}

func ParseGroupRecord(line string) (GroupRecord, error) {
	line = strings.TrimSpace(line)
	record := GroupRecord{
		RawData: line,
	}

	parts := strings.Split(line, ":")
	if len(parts) != 4 {
		return record, errors.New("unexpected field count")
	}

	record.Group = parts[0]
	record.Password = parts[1]
	record.MembersRaw = parts[3]
	if record.MembersRaw != "" {
		record.Members = strings.Split(record.MembersRaw, ",")
	}

	var err error
	record.GID, err = strconv.Atoi(parts[2])
	if err != nil {
		return record, fmt.Errorf("error parsing GID field - %s (%v)", parts[2], err)
	}

	return record, nil
}
