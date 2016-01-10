package core

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"os/exec"

	"github.com/glerchundi/renderizr/pkg/config"
	"github.com/glerchundi/renderizr/pkg/util"
	"github.com/golang/glog"
	"github.com/kelseyhightower/memkv"

	"sync"
)

// Template is the representation of a parsed template resource.
type Template struct {
	config        *config.TemplateConfig
	funcMap       map[string]interface{}
	store         memkv.Store
	doNoOp        bool
	keepStageFile bool
	useMutex      bool
	mutex         *sync.Mutex
}

func NewTemplate(config *config.TemplateConfig, doNoOp, keepStageFile, useMutex bool) *Template {
	store := memkv.New()
	funcMap := newFuncMap()
	for name, fn := range store.FuncMap {
		funcMap[name] = fn
	}

	return &Template{
		config: config,
		funcMap: funcMap,
		store: store,
		doNoOp: doNoOp,
		keepStageFile: keepStageFile,
		useMutex: useMutex,
		mutex: &sync.Mutex{},
	}
}

// Render is a convenience function that wraps calls to the three main
// tasks required to keep local configuration files in sync. First we
// stage a candidate configuration file, and finally sync things up.
// It returns an error if any fails.
func (t *Template) Render(kvs map[string]string) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	fileMode, err := t.getExpectedFileMode()
	if err != nil {
		return err
	}

	if err := t.setKVs(kvs); err != nil {
		return err
	}

	stageFile, err := t.createStageFile(fileMode)
	if err != nil {
		return err
	}

	if err := t.sync(stageFile, fileMode, t.doNoOp); err != nil {
		return err
	}

	return nil
}

// setFileMode sets the FileMode.
func (t *Template) getExpectedFileMode() (os.FileMode, error) {
	var fileMode os.FileMode = 0644
	if t.config.Mode == "" {
		if util.IsFileExist(t.config.Dest) {
			fi, err := os.Stat(t.config.Dest)
			if err != nil {
				return 0, err
			}
			fileMode = fi.Mode()
		}
	} else {
		mode, err := strconv.ParseUint(t.config.Mode, 0, 32)
		if err != nil {
			return 0, err
		}
		fileMode = os.FileMode(mode)
	}
	return fileMode, nil
}

// setKVs sets the Vars for template resource.
func (t *Template) setKVs(kvs map[string]string) error {
	t.store.Purge()
	for k, v := range kvs {
		t.store.Set(filepath.Join("/", strings.TrimPrefix(k, t.config.Prefix)), v)
	}
	return nil
}

// createStageFile stages the src configuration file by processing the src
// template and setting the desired owner, group, and mode. It also sets the
// StageFile for the template resource.
// It returns an error if any.
func (t *Template) createStageFile(fileMode os.FileMode) (*os.File, error) {
	glog.V(1).Infof("Using source template %s", t.config.Src)

	if !util.IsFileExist(t.config.Src) {
		return nil, errors.New("Missing template: " + t.config.Src)
	}

	glog.V(1).Infof("Compiling source template %s", t.config.Src)
	tmpl, err := template.New(path.Base(t.config.Src)).Funcs(t.funcMap).ParseFiles(t.config.Src)
	if err != nil {
		return nil, fmt.Errorf("Unable to process template %s, %s", t.config.Src, err)
	}

	// create TempFile in Dest directory to avoid cross-filesystem issues
	errorOcurred := true
	tempFile, err := ioutil.TempFile(filepath.Dir(t.config.Dest), "."+filepath.Base(t.config.Dest))
	if err != nil {
		return nil, err
	}
	defer func() {
		tempFile.Close()
		if !t.keepStageFile && errorOcurred {
			os.Remove(tempFile.Name())
		}
	}()

	if err = tmpl.Execute(tempFile, nil); err != nil {
		return nil, err
	}

	// Set the owner, group, and mode on the stage file now to make it easier to
	// compare against the destination configuration file later.
	err = os.Chmod(tempFile.Name(), fileMode)
	if err != nil {
		return nil, err
	}

	err = os.Chown(tempFile.Name(), t.config.Uid, t.config.Gid)
	if err != nil {
		return nil, err
	}

	errorOcurred = false
	return tempFile, nil
}

// sync compares the staged and dest config files and attempts to sync them
// if they differ. sync will run a config check command if set before
// overwriting the target config file. Finally, sync will run a reload command
// if set to have the application or service pick up the changes.
// It returns an error if any.
func (t *Template) sync(stageFile *os.File, fileMode os.FileMode, doNoOp bool) error {
	stageFileName := stageFile.Name()
	if !t.keepStageFile {
		defer os.Remove(stageFileName)
	}

	glog.V(1).Infof("Comparing candidate config to %s", t.config.Dest)
	ok, err := util.IsSameConfig(stageFileName, t.config.Dest)
	if err != nil {
		glog.Error(err)
		return err
	}

	if doNoOp {
		glog.Warningf("Noop mode enabled. %s will not be modified", t.config.Dest)
		return nil
	}

	if !ok {
		glog.Infof("Target config %s out of sync", t.config.Dest)

		if t.config.CheckCmd != "" {
			if err := t.check(stageFileName); err != nil {
				return errors.New("Config check failed: " + err.Error())
			}
		}

		glog.V(1).Infof("Overwriting target config %s", t.config.Dest)

		err := os.Rename(stageFileName, t.config.Dest)
		if err != nil {
			if strings.Contains(err.Error(), "device or resource busy") {
				glog.V(1).Infof("Rename failed - target is likely a mount.config. Trying to write instead")
				// try to open the file and write to it
				var contents []byte
				var rerr error
				contents, rerr = ioutil.ReadFile(stageFileName)
				if rerr != nil {
					return rerr
				}
				err := ioutil.WriteFile(t.config.Dest, contents, fileMode)
				// make sure owner and group match the temp file, in case the file was created with WriteFile
				os.Chown(t.config.Dest, t.config.Uid, t.config.Gid)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}

		if t.config.ReloadCmd != "" {
			if err := t.reload(); err != nil {
				return err
			}
		}

		glog.Infof("Target config %s has been updated", t.config.Dest)
	} else {
		glog.V(1).Infof("Target config %s in sync", t.config.Dest)
	}

	return nil
}

// check executes the check command to validate the staged config file. The
// command is modified so that any references to src template are substituted
// with a string representing the full path of the staged file. This allows the
// check to be run on the staged file before overwriting the destination config
// file.
// It returns nil if the check command returns 0 and there are no other errors.
func (t *Template) check(stageFileName string) error {
	tmpl, err := template.New("checkcmd").Parse(t.config.CheckCmd)
	if err != nil {
		return err
	}

	var cmdBuffer bytes.Buffer
	if err := tmpl.Execute(&cmdBuffer, stageFileName); err != nil {
		return err
	}

	return t.exec(cmdBuffer.String())
}

// reload executes the reload command.
// It returns nil if the reload command returns 0.
func (t *Template) reload() error {
	return t.exec(t.config.ReloadCmd)
}

func (t *Template) exec(cmd string) error {
	glog.V(1).Infof("Running %s", cmd)

	c := exec.Command("/bin/sh", "-c", cmd)
	output, err := c.CombinedOutput()
	if err != nil {
		glog.Errorf("%q", string(output))
		return err
	}

	glog.V(1).Infof("%q", string(output))

	return nil
}