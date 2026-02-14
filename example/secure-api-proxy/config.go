package main

// Config represents the RBAC and proxy configuration,
// adapted from the user's ASP.Net implementation at https://github.com/scaleoutsean/solidfire-wac-gateway?tab=readme-ov-file#quick-start
type Config struct {
	GlobalAdminRoles []string                 `yaml:"global_admin_roles"`
	AccessRules      map[string]AccessControl `yaml:"access_rules"`
	TenantOptions    TenantOptions            `yaml:"tenant_options"`
	Clusters         map[string]ClusterConfig `yaml:"clusters"`
	Server           ServerConfig             `yaml:"server"`
	Logging          LoggingConfig            `yaml:"logging"`
}

type AccessControl struct {
	ActionRoles map[string][]string `yaml:"action_roles"` // e.g. "Create": ["SFADMINS"]
}

type TenantOptions struct {
	AllowedTenants []int64 `yaml:"allowed_tenants"`
}

type ClusterConfig struct {
	Endpoint string `yaml:"endpoint"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type ServerConfig struct {
	ListenAddr string `yaml:"listen_addr"`
	CertFile   string `yaml:"cert_file"`
	KeyFile    string `yaml:"key_file"`
	UsePQC     bool   `yaml:"use_pqc"` // Enable Post-Quantum Cryptography
}

type LoggingConfig struct {
	Level      string `yaml:"level"`       // debug, info, warn, error
	Format     string `yaml:"format"`      // text, json
	Output     string `yaml:"output"`      // stdout, file
	FilePath   string `yaml:"file_path"`   // if output is file
	SyslogAddr string `yaml:"syslog_addr"` // e.g. "logs.example.com:6514" for TLS Syslog
}
