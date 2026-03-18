package logstore_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thegeeklab/renovate-operator/internal/logstore"
)

var _ = Describe("FileStore", func() {
	var (
		store   *logstore.FileStore
		tempDir string
		ctx     context.Context
	)

	BeforeEach(func() {
		var err error

		tempDir, err = os.MkdirTemp("", "filestore-test-*")
		Expect(err).NotTo(HaveOccurred())

		store = logstore.NewFileStore(tempDir)
		ctx = context.Background()
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tempDir)).To(Succeed())
	})

	Describe("SaveLog", func() {
		It("should successfully save a log to the filesystem", func() {
			logContent := "this is a test log\nline 2"
			reader := strings.NewReader(logContent)

			err := store.SaveLog(ctx, "ns1", "runner", "repo1", "job-1", reader)
			Expect(err).NotTo(HaveOccurred())

			expectedPath := filepath.Join(tempDir, "ns1", "runner", "repo1", "job-1.log")
			Expect(expectedPath).To(BeAnExistingFile())

			content, err := os.ReadFile(expectedPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal(logContent))
		})
	})

	Describe("GetLog", func() {
		It("should successfully retrieve an existing log", func() {
			logContent := "hello world"
			err := store.SaveLog(ctx, "ns1", "runner", "repo1", "job-1", strings.NewReader(logContent))
			Expect(err).NotTo(HaveOccurred())

			reader, err := store.GetLog(ctx, "ns1", "runner", "repo1", "job-1")
			Expect(err).NotTo(HaveOccurred())

			defer reader.Close()

			content, err := io.ReadAll(reader)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal(logContent))
		})

		It("should return ErrLogNotFound when the log does not exist", func() {
			reader, err := store.GetLog(ctx, "ns1", "runner", "repo1", "non-existent-job")
			Expect(err).To(MatchError(logstore.ErrLogNotFound))
			Expect(reader).To(BeNil())
		})
	})

	Describe("ListLogs", func() {
		It("should return an empty slice if the directory does not exist", func() {
			logs, err := store.ListLogs(ctx, "ns1", "runner", "repo1")
			Expect(err).NotTo(HaveOccurred())
			Expect(logs).To(BeEmpty())
		})

		It("should list existing logs sorted from newest to oldest", func() {
			Expect(store.SaveLog(ctx, "ns1", "runner", "repo1", "job-1", strings.NewReader("log 1"))).To(Succeed())
			Expect(store.SaveLog(ctx, "ns1", "runner", "repo1", "job-2", strings.NewReader("log 22"))).To(Succeed())
			Expect(store.SaveLog(ctx, "ns1", "runner", "repo1", "job-3", strings.NewReader("log 333"))).To(Succeed())

			dir := filepath.Join(tempDir, "ns1", "runner", "repo1")

			now := time.Now()
			Expect(os.Chtimes(filepath.Join(dir, "job-1.log"), now, now.Add(-10*time.Minute))).To(Succeed())
			Expect(os.Chtimes(filepath.Join(dir, "job-2.log"), now, now.Add(-5*time.Minute))).To(Succeed())
			Expect(os.Chtimes(filepath.Join(dir, "job-3.log"), now, now)).To(Succeed())

			logs, err := store.ListLogs(ctx, "ns1", "runner", "repo1")
			Expect(err).NotTo(HaveOccurred())
			Expect(logs).To(HaveLen(3))

			Expect(logs[0].JobName).To(Equal("job-3"))
			Expect(logs[1].JobName).To(Equal("job-2"))
			Expect(logs[2].JobName).To(Equal("job-1"))

			Expect(logs[0].SizeBytes).To(Equal(int64(7)))
			Expect(logs[0].Namespace).To(Equal("ns1"))
			Expect(logs[0].Component).To(Equal("runner"))
			Expect(logs[0].Owner).To(Equal("repo1"))
		})

		It("should ignore directories and non-log files", func() {
			Expect(store.SaveLog(ctx, "ns1", "runner", "repo1", "job-1", strings.NewReader("log 1"))).To(Succeed())

			dir := filepath.Join(tempDir, "ns1", "runner", "repo1")

			Expect(os.WriteFile(filepath.Join(dir, "random.txt"), []byte("not a log"), 0o644)).To(Succeed())

			Expect(os.Mkdir(filepath.Join(dir, "directory.log"), 0o755)).To(Succeed())

			logs, err := store.ListLogs(ctx, "ns1", "runner", "repo1")
			Expect(err).NotTo(HaveOccurred())

			Expect(logs).To(HaveLen(1))
			Expect(logs[0].JobName).To(Equal("job-1"))
		})
	})

	Describe("DeleteLog", func() {
		It("should successfully delete an existing log", func() {
			Expect(store.SaveLog(ctx, "ns1", "runner", "repo1", "job-1", strings.NewReader("log 1"))).To(Succeed())

			expectedPath := filepath.Join(tempDir, "ns1", "runner", "repo1", "job-1.log")
			Expect(expectedPath).To(BeAnExistingFile())

			err := store.DeleteLog(ctx, "ns1", "runner", "repo1", "job-1")
			Expect(err).NotTo(HaveOccurred())

			Expect(expectedPath).NotTo(BeAnExistingFile())
		})

		It("should not return an error when deleting a non-existent log", func() {
			err := store.DeleteLog(ctx, "ns1", "runner", "repo1", "non-existent-job")
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
