package secrets

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func Get(ctx context.Context, sel *SecretKeySelector) (*corev1.Secret, error) {
	rc, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	cli, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return nil, err
	}

	return cli.CoreV1().Secrets(sel.Namespace).Get(ctx, sel.Name, metav1.GetOptions{})
}

func Create(ctx context.Context, secret *corev1.Secret) error {
	rc, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	cli, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return err
	}

	_, err = cli.CoreV1().Secrets(secret.GetNamespace()).Create(ctx, secret, metav1.CreateOptions{})
	return err
}

func Update(ctx context.Context, secret *corev1.Secret) error {
	rc, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	cli, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return err
	}

	_, err = cli.CoreV1().Secrets(secret.GetNamespace()).Update(ctx, secret, metav1.UpdateOptions{})
	return err
}

func Delete(ctx context.Context, sel *SecretKeySelector) error {
	rc, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	cli, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return err
	}

	return cli.CoreV1().Secrets(sel.Namespace).Delete(ctx, sel.Name, metav1.DeleteOptions{})
}

// SecretKeySelector and its methods remain the same
type SecretKeySelector struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
}

func (in *SecretKeySelector) DeepCopy() *SecretKeySelector {
	if in == nil {
		return nil
	}
	out := new(SecretKeySelector)
	in.DeepCopyInto(out)
	return out
}

func (in *SecretKeySelector) DeepCopyInto(out *SecretKeySelector) {
	*out = *in
}
