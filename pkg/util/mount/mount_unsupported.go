// +build !linux

/*
Copyright 2014 Google Inc. All rights reserved.

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

package mount

const FlagBind = 0
const FlagReadOnly = 0

type Mounter struct{}

func (mounter *Mounter) Mount(source string, target string, fstype string, flags uintptr, data string) error {
	return nil
}

func (mounter *Mounter) Unmount(target string, flags int) error {
	return nil
}

func (mounter *Mounter) List() ([]MountPoint, error) {
	return []MountPoint{}, nil
}
