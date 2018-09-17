package google

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/appscode/go/types"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
	gdir "google.golang.org/api/admin/directory/v1"
	"k8s.io/api/apps/v1beta1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type Options struct {
	ServiceAccountJsonFile string
	AdminEmail             string
	ClientID               string
	ClientSecret           string
	jwtConfig              *jwt.Config
}

func NewOptions() Options {
	return Options{
		// https://developers.google.com/identity/protocols/OAuth2InstalledApp
		ClientID: os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
	}
}

func (o *Options) Configure() error {
	if o.ServiceAccountJsonFile != "" {
		sa, err := ioutil.ReadFile(o.ServiceAccountJsonFile)
		if err != nil {
			return errors.Wrapf(err, "failed to load service account json file %s", o.ServiceAccountJsonFile)
		}

		o.jwtConfig, err = google.JWTConfigFromJSON(sa, gdir.AdminDirectoryGroupReadonlyScope)
		if err != nil {
			return errors.Wrapf(err, "failed to create JWT config from service account json file %s", o.ServiceAccountJsonFile)
		}

		// https://admin.google.com/ManageOauthClients
		// ref: https://developers.google.com/admin-sdk/directory/v1/guides/delegation
		// Note: Only users with access to the Admin APIs can access the Admin SDK Directory API, therefore your service account needs to impersonate one of those users to access the Admin SDK Directory API.
		o.jwtConfig.Subject = o.AdminEmail
	}

	return nil
}

func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.ServiceAccountJsonFile, "google.sa-json-file", o.ServiceAccountJsonFile, "Path to Google service account json file")
	fs.StringVar(&o.AdminEmail, "google.admin-email", o.AdminEmail, "Email of G Suite administrator")
	fs.StringVar(&o.ClientID, "google.client-id", o.ClientID, "OAuth2 application client ID to use")
	fs.StringVar(&o.ClientSecret, "google.client-secret", o.ClientSecret, "OAuth2 application client secret to use")
}

func (o *Options) Validate() []error {
	var errs []error
	if o.ServiceAccountJsonFile == "" {
		errs = append(errs, errors.New("google.sa-json-file must be non-empty"))
	}
	if o.AdminEmail == "" {
		errs = append(errs, errors.New("google.admin-email must be non-empty"))
	}
	if o.ClientSecret == "" {
		errs = append(errs, errors.New("client secret must be non-empty"))
	}
	if o.ClientID == "" {
		errs = append(errs, errors.New("client-id must be non-empty"))
	}
	return errs
}

func (o Options) Apply(d *v1beta1.Deployment) (extraObjs []runtime.Object, err error) {
	container := d.Spec.Template.Spec.Containers[0]

	// create auth secret
	sa, err := ioutil.ReadFile(o.ServiceAccountJsonFile)
	if err != nil {
		return nil, err
	}
	authSecret := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "guard-google-auth",
			Namespace: d.Namespace,
			Labels:    d.Labels,
		},
		Data: map[string][]byte{
			"sa.json": sa,
		},
	}
	extraObjs = append(extraObjs, authSecret)

	// mount auth secret into deployment
	volMount := core.VolumeMount{
		Name:      authSecret.Name,
		MountPath: "/etc/guard/auth/google",
	}
	container.VolumeMounts = append(container.VolumeMounts, volMount)

	vol := core.Volume{
		Name: authSecret.Name,
		VolumeSource: core.VolumeSource{
			Secret: &core.SecretVolumeSource{
				SecretName:  authSecret.Name,
				DefaultMode: types.Int32P(0555),
			},
		},
	}
	d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, vol)

	// use auth secret in container[0] args
	container.Env = append(container.Env, core.EnvVar{
		Name: "GOOGLE_CLIENT_SECRET",
		ValueFrom: &core.EnvVarSource{
			SecretKeyRef: &core.SecretKeySelector{
				LocalObjectReference: core.LocalObjectReference{
					Name: "google-oidc-credentials";
				},
				Key: "client-secret",
			},
		},
	})

	args := container.Args
	if o.ClientID != "" {
		args = append(args, fmt.Sprintf("--google.client-id=%s", o.ClientID))
	}
	if o.ServiceAccountJsonFile != "" {
		args = append(args, "--google.sa-json-file=/etc/guard/auth/google/sa.json")
	}
	if o.AdminEmail != "" {
		args = append(args, fmt.Sprintf("--google.admin-email=%s", o.AdminEmail))
	}

	container.Args = args
	d.Spec.Template.Spec.Containers[0] = container

	return extraObjs, nil
}
