module github.com/replicatedhq/unfork

go 1.12

require (
	cloud.google.com/go v0.38.0 // indirect
	github.com/Masterminds/semver v1.4.2
	github.com/ahmetalpbalkan/go-cursor v0.0.0-20131010032410-8136607ea412
	github.com/chzyer/logex v1.1.11-0.20160617073814-96a4d311aa9b // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/gizak/termui/v3 v3.1.0
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/nicksnyder/go-i18n v2.0.2+incompatible // indirect
	github.com/nicksnyder/go-i18n/v2 v2.0.2 // indirect
	github.com/pkg/errors v0.8.1
	github.com/replicatedhq/kots v0.5.1-0.20190904162055-2988cce69f1c
	github.com/spf13/cobra v0.0.5
	github.com/spf13/viper v1.4.0
	github.com/stretchr/testify v1.3.0
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45 // indirect
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20190516230258-a675ac48af67
	k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/cli-runtime v0.0.0-20190516231937-17bc0b7fcef5
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/helm v2.14.3+incompatible
	k8s.io/klog v0.4.0 // indirect
	k8s.io/kube-openapi v0.0.0-20190816220812-743ec37842bf // indirect
	k8s.io/utils v0.0.0-20190809000727-6c36bc71fc4a // indirect
	sigs.k8s.io/kustomize v2.0.3+incompatible
	sigs.k8s.io/kustomize/v3 v3.1.0
)

replace github.com/nicksnyder/go-i18n v2.0.2+incompatible => github.com/nicksnyder/go-i18n/v2 v2.0.2
