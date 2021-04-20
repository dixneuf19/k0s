/*
Copyright 2021 k0s authors

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
package restore

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.etcd.io/etcd/clientv3/snapshot"
	"go.uber.org/zap"
)

type CmdOpts config.CLIOptions

func NewRestoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "restore k0s state from given backup archive. Must be run as root (or with sudo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := CmdOpts(config.GetCmdOpts())
			if len(args) != 1 {
				return fmt.Errorf("path to backup archive expected")
			}
			cfg, err := config.GetYamlFromFile(c.CfgFile, c.K0sVars)
			if err != nil {
				return err
			}

			c.ClusterConfig = cfg
			return c.restore(args[0])
		},
		PreRunE: preRunValidateConfig,
	}

	cmd.SilenceUsage = true
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	return cmd
}

func (c *CmdOpts) restore(path string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("this command must be run as root")
	}

	if !util.FileExists(path) {
		return fmt.Errorf("given file %s does not exist", path)
	}

	if !util.DirExists(c.K0sVars.DataDir) {
		if err := util.InitDirectory(c.K0sVars.DataDir, constant.DataDirMode); err != nil {
			return err
		}
	}

	if err := util.ExtractArchive(path, c.K0sVars.DataDir); err != nil {
		return err
	}

	// TODO check if the config actually says we're using etcd
	if c.ClusterConfig.Spec.Storage.Type == v1beta1.EtcdStorageType {
		if err := c.restoreEtcd(); err != nil {
			return err
		}
	} else {
		logrus.Warnf("database is NOT restored automatically, you must restore it manually")
	}

	logrus.Infof("k0s restored succesfully from %s", path)
	return nil
}

func (c *CmdOpts) restoreEtcd() error {
	snapshotPath := filepath.Join(c.K0sVars.DataDir, "etcd-snapshot.db")
	if !util.FileExists(snapshotPath) {
		return fmt.Errorf("etcd snapshot not found at %s", snapshotPath)
	}

	// disable etcd's logging
	lg := zap.NewNop()
	m := snapshot.NewV3(lg)
	name, err := os.Hostname()
	if err != nil {
		return err
	}
	peerURL := fmt.Sprintf("https://%s:2380", c.ClusterConfig.Spec.Storage.Etcd.PeerAddress)
	restoreConfig := snapshot.RestoreConfig{
		SnapshotPath:   snapshotPath,
		OutputDataDir:  c.K0sVars.EtcdDataDir,
		PeerURLs:       []string{peerURL},
		Name:           name,
		InitialCluster: fmt.Sprintf("%s=%s", name, peerURL),
	}

	err = m.Restore(restoreConfig)
	if err != nil {
		return err
	}

	return nil
}

// TODO Need to move to some common place, now just copied multiple times :(
func preRunValidateConfig(cmd *cobra.Command, args []string) error {
	c := CmdOpts(config.GetCmdOpts())
	_, err := config.ValidateYaml(c.CfgFile, c.K0sVars)
	if err != nil {
		return err
	}
	return nil
}
