/*
リファクタリング中
処理をslacklog packageに移動していく。
一旦、必要な処理はすべてslacklog packageから一時的にエクスポートするか、このファ
イル内で定義している。
*/

package subcmd

import (
	"fmt"
	"os"
	"path/filepath"

	cli "github.com/urfave/cli/v2"
	"github.com/vim-jp/slacklog-generator/internal/slacklog"
)

var FilesFlags = []cli.Flag{
	&cli.StringFlag{
		Name: "indir",
		Usage: "slacklog_data dir",
		Value: filepath.Join("_logdata", "slacklog_data"),
	},
	&cli.StringFlag{
		Name: "outdir",
		Usage: "files download target dir",
		Value: filepath.Join("_logdata", "files"),
	},
}

// DownloadFiles downloads and saves files which attached to message.
func DownloadFiles(c *cli.Context) error {
	slackToken := os.Getenv("SLACK_TOKEN")
	if slackToken == "" {
		return fmt.Errorf("$SLACK_TOKEN required")
	}

	logDir := filepath.Clean(c.String("indir"))
	filesDir := filepath.Clean(c.String("outdir"))

	s, err := slacklog.NewLogStore(logDir, &slacklog.Config{Channels: []string{"*"}})
	if err != nil {
		return err
	}

	d := slacklog.NewDownloader(slackToken)

	go generateMessageFileTargets(d, s, filesDir)

	err = d.Wait()
	if err != nil {
		return err
	}
	return nil
}

func generateMessageFileTargets(d *slacklog.Downloader, s *slacklog.LogStore, outputDir string) {
	defer d.CloseQueue()
	channels := s.GetChannels()
	for _, channel := range channels {
		msgs, err := s.GetAllMessages(channel.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to get messages on %s channel: %s", channel.Name, err)
			return
		}

		for _, msg := range msgs {
			if !msg.Upload {
				continue
			}
			for _, f := range msg.Files {
				// 基本的に msg.Upload の判定で弾けるはずだが、
				// 複数のファイルが含まれていた場合が不明。
				// 念のためこちらでもチェックして弾くようにしておく
				if !f.IsSlackHosted() {
					continue
				}

				targetDir := filepath.Join(outputDir, f.ID)
				err := os.MkdirAll(targetDir, 0777)
				if err != nil {
					fmt.Fprintf(os.Stderr, "failed to create %s directory: %s", targetDir, err)
					return
				}

				for url, suffix := range f.DownloadURLsAndSuffixes() {
					if url == "" {
						continue
					}
					d.QueueDownloadRequest(
						url,
						filepath.Join(targetDir, f.DownloadFilename(url, suffix)),
						true,
					)
				}
			}
		}
	}
}
