package unforker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	upstreamFilesFixture = map[string][]byte{
		"deployment.yaml": []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
        - name: nginx
          image: nginx:1.7.9
          ports:
           - containerPort: 80
`),

		"database.yaml": []byte(`apiVersion: databases.schemahero.io/v1alpha2
kind: Database
metadata:
  name: rds-postgres
  namespace: default
connection:
  postgres:
    uri:
      valueFrom:
        secretKeyRef:
          key: uri
          name: rds-postgres
`),
		"deployment-2.yaml": []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: deployment
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
        - name: nginx
          image: nginx:1.7.9
          ports:
           - containerPort: 80
`),
	}
)

func Test_findMatchingUpstreamPath(t *testing.T) {
	tests := []struct {
		name            string
		upstreamFiles   map[string][]byte
		forkedContent   []byte
		expected        string
		expectedPrefix  string
		suspectedPrefix string
	}{
		{
			name:          "find a deployment",
			upstreamFiles: upstreamFilesFixture,
			forkedContent: []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  namespace: default`),
			expected: "deployment.yaml",
		},
		{
			// there are two deployments that have a name 'myprefixed-nginx-deployment' ends with
			// we should pick the right one
			name:          "find a deployment by prefix",
			upstreamFiles: upstreamFilesFixture,
			forkedContent: []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: myprefixed-nginx-deployment
  namespace: default`),
			expected:        "deployment.yaml",
			expectedPrefix:  "myprefixed-",
			suspectedPrefix: "myprefixed-",
		},
		{
			name:          "find a nonexistent deployment",
			upstreamFiles: upstreamFilesFixture,
			forkedContent: []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: this-is-a-unique-name
  namespace: default`),
			expected: "",
		},
		{
			name:          "find a deployment by suspected prefix",
			upstreamFiles: upstreamFilesFixture,
			forkedContent: []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: myprefixed-nginx-deployment
  namespace: default`),
			expected:        "deployment-2.yaml",
			expectedPrefix:  "myprefixed-nginx-",
			suspectedPrefix: "myprefixed-nginx-",
		},
		{
			name:          "find a database by incorrect suspected prefix",
			upstreamFiles: upstreamFilesFixture,
			forkedContent: []byte(`apiVersion: databases.schemahero.io/v1alpha2
kind: Database
metadata:
  name: something-rds-postgres
  namespace: default`),
			expected:        "database.yaml",
			expectedPrefix:  "something-",
			suspectedPrefix: "myprefixed-",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			actual, prefix, err := findMatchingUpstreamPath(test.upstreamFiles, test.forkedContent, test.suspectedPrefix)
			req.NoError(err)
			req.Equal(test.expected, actual)
			req.Equal(test.expectedPrefix, prefix)
		})
	}
}
