package logger

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLog(t *testing.T) {
	testcases := []struct {
		description     string
		path            string
		object          Data
		preliminaryFunc func() error
		afterFunc       func() error
	}{
		{
			description: "Logs the object correctly",
			path:        "log.txt",
			object: Data{
				Time: time.Now(),
				Object: &corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "bar",
						Labels: map[string]string{
							"system-autoscaler/node": "node",
						},
					},
					Data: map[string][]byte{
						"my-data": []byte("my-value"),
					},
					Type: corev1.SecretTypeOpaque,
				},
			},
			preliminaryFunc: func() error {
				return nil
			},
			afterFunc: func() error {
				return nil
			},
		},
		{
			description: "Support a relative path",
			path:        "folder/log.txt",
			object: Data{
				Time: time.Now(),
				Object: &corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "bar",
						Labels: map[string]string{
							"system-autoscaler/node": "node",
						},
					},
					Data: map[string][]byte{
						"my-data": []byte("my-value"),
					},
					Type: corev1.SecretTypeOpaque,
				},
			},
			preliminaryFunc: func() error {
				return os.Mkdir("folder", 0777)
			},
			afterFunc: func() error {
				return os.Remove("folder")
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			err := tt.preliminaryFunc()

			require.Nil(t, err)

			l, err := NewFileLogger(tt.path)

			require.Nil(t, err)

			err = l.Log(tt.object.Object)

			require.Nil(t, err)

			actualData, err := ioutil.ReadFile(tt.path)

			require.Nil(t, err)

			actual := Data{
				Object: &corev1.Secret{},
			}

			err = json.Unmarshal(actualData, &actual)

			require.Nil(t, err)

			require.Equal(t, tt.object.Object, actual.Object)

			err = os.Remove(tt.path)

			require.Nil(t, err)

			err = tt.afterFunc()

			require.Nil(t, err)
		})
	}
}
