/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Modified from the original source (available at
// https://github.com/kubernetes-sigs/controller-runtime/tree/v0.5.0/pkg/cache)

package internal

import (
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// InformersMap create and caches Informers for (runtime.Object, schema.GroupVersionKind) pairs.
// It uses a standard parameter codec constructed based on the given generated Scheme.
type InformersMap struct {
	// we abstract over the details of structured vs unstructured with the specificInformerMaps

	structured   *specificInformersMap
	unstructured *specificInformersMap

	// Scheme maps runtime.Objects to GroupVersionKinds
	Scheme *runtime.Scheme
}

// NewInformersMap creates a new InformersMap that can create informers for
// both structured and unstructured objects.
func NewInformersMap(config *rest.Config,
	scheme *runtime.Scheme,
	mapper meta.RESTMapper,
	resync time.Duration,
	namespace string) *InformersMap {

	return &InformersMap{
		structured:   newStructuredInformersMap(config, scheme, mapper, resync, namespace),
		unstructured: newUnstructuredInformersMap(config, scheme, mapper, resync, namespace),

		Scheme: scheme,
	}
}

// Start calls Run on each of the informers and sets started to true.  Blocks on the stop channel.
func (m *InformersMap) Start(stop <-chan struct{}) error {
	go m.structured.Start(stop)
	go m.unstructured.Start(stop)
	<-stop
	return nil
}

// WaitForCacheSync waits until all the caches have been started and synced.
func (m *InformersMap) WaitForCacheSync(stop <-chan struct{}) bool {
	syncedFuncs := append([]cache.InformerSynced(nil), m.structured.HasSyncedFuncs()...)
	syncedFuncs = append(syncedFuncs, m.unstructured.HasSyncedFuncs()...)

	if !m.structured.waitForStarted(stop) {
		return false
	}
	if !m.unstructured.waitForStarted(stop) {
		return false
	}
	return cache.WaitForCacheSync(stop, syncedFuncs...)
}

// Get will create a new Informer and add it to the map of InformersMap if none exists.  Returns
// the Informer from the map.
func (m *InformersMap) Get(gvk schema.GroupVersionKind, obj runtime.Object) (bool, *MapEntry, error) {
	_, isUnstructured := obj.(*unstructured.Unstructured)
	_, isUnstructuredList := obj.(*unstructured.UnstructuredList)
	isUnstructured = isUnstructured || isUnstructuredList

	if isUnstructured {
		return m.unstructured.Get(gvk, obj, true)
	}

	return m.structured.Get(gvk, obj, true)
}

// GetNonBlocking will create a new Informer and add it to the map of InformersMap if none exists.
// Returns the Informer from the map.
// This method differs from Get() in that it will not block for cache sync when an informer is first instantiated.
func (m *InformersMap) GetNonBlocking(gvk schema.GroupVersionKind, obj runtime.Object) (bool, *MapEntry, error) {
	_, isUnstructured := obj.(*unstructured.Unstructured)
	_, isUnstructuredList := obj.(*unstructured.UnstructuredList)
	isUnstructured = isUnstructured || isUnstructuredList

	if isUnstructured {
		return m.unstructured.Get(gvk, obj, false)
	}

	return m.structured.Get(gvk, obj, false)
}

// Remove will remove an new Informer from the InformersMap and stop it if it exists.
func (m *InformersMap) Remove(gvk schema.GroupVersionKind, obj runtime.Object) {
	_, isUnstructured := obj.(*unstructured.Unstructured)
	_, isUnstructuredList := obj.(*unstructured.UnstructuredList)
	isUnstructured = isUnstructured || isUnstructuredList

	switch {
	case isUnstructured:
		m.unstructured.Remove(gvk)
	default:
		m.structured.Remove(gvk)
	}
}

// newStructuredInformersMap creates a new InformersMap for structured objects.
func newStructuredInformersMap(config *rest.Config, scheme *runtime.Scheme, mapper meta.RESTMapper, resync time.Duration, namespace string) *specificInformersMap {
	return newSpecificInformersMap(config, scheme, mapper, resync, namespace, createStructuredListWatch)
}

// newUnstructuredInformersMap creates a new InformersMap for unstructured objects.
func newUnstructuredInformersMap(config *rest.Config, scheme *runtime.Scheme, mapper meta.RESTMapper, resync time.Duration, namespace string) *specificInformersMap {
	return newSpecificInformersMap(config, scheme, mapper, resync, namespace, createUnstructuredListWatch)
}
