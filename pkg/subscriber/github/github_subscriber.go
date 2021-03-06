// Copyright 2019 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package github

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	appv1alpha1 "github.com/IBM/multicloud-operators-subscription/pkg/apis/app/v1alpha1"
	kubesynchronizer "github.com/IBM/multicloud-operators-subscription/pkg/synchronizer/kubernetes"
)

type itemmap map[types.NamespacedName]*SubscriberItem

// Subscriber - information to run namespace subscription
type Subscriber struct {
	itemmap
	manager      manager.Manager
	synchronizer *kubesynchronizer.KubeSynchronizer
	syncinterval int
}

var defaultSubscriber *Subscriber

var githubk8ssyncsource = "subgbk8s-"
var githubhelmsyncsource = "subgbhelm-"

// Add does nothing for namespace subscriber, it generates cache for each of the item
func Add(mgr manager.Manager, hubconfig *rest.Config, syncid *types.NamespacedName, syncinterval int) error {
	// No polling, use cache. Add default one for cluster namespace
	var err error

	klog.V(5).Info("Setting up default github subscriber on ", syncid)

	sync := kubesynchronizer.GetDefaultSynchronizer()
	if sync == nil {
		err = kubesynchronizer.Add(mgr, hubconfig, syncid, syncinterval)
		if err != nil {
			klog.Error("Failed to initialize synchronizer for default namespace channel with error:", err)
			return err
		}

		sync = kubesynchronizer.GetDefaultSynchronizer()
	}

	if err != nil {
		klog.Error("Failed to create synchronizer for subscriber with error:", err)
		return err
	}

	defaultSubscriber = CreateGitHubSubscriber(hubconfig, mgr.GetScheme(), mgr, sync, syncinterval)
	if defaultSubscriber == nil {
		errmsg := "failed to create default namespace subscriber"

		return errors.New(errmsg)
	}

	return nil
}

// SubscribeItem subscribes a subscriber item with namespace channel
func (ghs *Subscriber) SubscribeItem(subitem *appv1alpha1.SubscriberItem) error {
	if ghs.itemmap == nil {
		ghs.itemmap = make(map[types.NamespacedName]*SubscriberItem)
	}

	itemkey := types.NamespacedName{Name: subitem.Subscription.Name, Namespace: subitem.Subscription.Namespace}
	klog.V(2).Info("subscribeItem ", itemkey)

	ghssubitem, ok := ghs.itemmap[itemkey]

	if !ok {
		ghssubitem = &SubscriberItem{}
		ghssubitem.syncinterval = ghs.syncinterval
		ghssubitem.synchronizer = ghs.synchronizer
	}

	subitem.DeepCopyInto(&ghssubitem.SubscriberItem)
	ghssubitem.commitID = ""

	ghs.itemmap[itemkey] = ghssubitem

	ghssubitem.Start()

	return nil
}

// UnsubscribeItem uhrsubscribes a namespace subscriber item
func (ghs *Subscriber) UnsubscribeItem(key types.NamespacedName) error {
	klog.V(2).Info("UnsubscribeItem ", key)

	subitem, ok := ghs.itemmap[key]

	if ok {
		subitem.Stop()
		delete(ghs.itemmap, key)
		ghs.synchronizer.CleanupByHost(key, githubk8ssyncsource+key.String())
		ghs.synchronizer.CleanupByHost(key, githubhelmsyncsource+key.String())
	}

	return nil
}

// GetDefaultSubscriber - returns the defajlt namespace subscriber
func GetDefaultSubscriber() appv1alpha1.Subscriber {
	return defaultSubscriber
}

// CreateGitHubSubscriber - create github subscriber with config to hub cluster, scheme of hub cluster and a syncrhonizer to local cluster
func CreateGitHubSubscriber(config *rest.Config, scheme *runtime.Scheme, mgr manager.Manager,
	kubesync *kubesynchronizer.KubeSynchronizer, syncinterval int) *Subscriber {
	if config == nil || kubesync == nil {
		klog.Error("Can not create github subscriber with config: ", config, " kubenetes synchronizer: ", kubesync)
		return nil
	}

	githubsubscriber := &Subscriber{
		manager:      mgr,
		synchronizer: kubesync,
	}

	githubsubscriber.itemmap = make(map[types.NamespacedName]*SubscriberItem)
	githubsubscriber.syncinterval = syncinterval

	return githubsubscriber
}
