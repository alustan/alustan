package git

import (
	"log"
	"os"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"golang.org/x/crypto/ssh"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	gitssh "gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
)

// CloneOrPullRepo clones the repository if it does not exist, or pulls the latest changes if it does.
// It uses the SSH key for authentication if provided.
func CloneOrPullRepo(repoURL, branch, repoDir, sshKey string) error {
	var repo *git.Repository
	var err error
	var auth transport.AuthMethod

	log.Printf("Starting CloneOrPullRepo for repo: %s, branch: %s, directory: %s", repoURL, branch, repoDir)

	// If an SSH key is provided, set up the authentication
	if sshKey != "" {
		log.Println("Setting up SSH authentication")
		signer, err := ssh.ParsePrivateKey([]byte(sshKey))
		if err != nil {
			log.Printf("Failed to parse SSH key: %v", err)
			return err
		}

		auth = &gitssh.PublicKeys{
			User:   "git",
			Signer: signer,
		}
	}

	if _, err = os.Stat(repoDir); os.IsNotExist(err) {
		log.Printf("Directory %s does not exist. Cloning repository...", repoDir)
		// Clone the repository
		repo, err = git.PlainClone(repoDir, false, &git.CloneOptions{
			URL:           repoURL,
			ReferenceName: plumbing.NewBranchReferenceName(branch),
			Auth:          auth,
		})
		if err != nil {
			log.Printf("Failed to clone repository: %v", err)
			return err
		}
		log.Println("Repository cloned successfully.")
	} else {
		log.Printf("Directory %s exists. Opening repository...", repoDir)
		// Open the existing repository and pull the latest changes
		repo, err = git.PlainOpen(repoDir)
		if err != nil {
			log.Printf("Failed to open repository: %v", err)
			return err
		}

		worktree, err := repo.Worktree()
		if err != nil {
			log.Printf("Failed to get worktree: %v", err)
			return err
		}

		log.Println("Pulling latest changes from repository...")
		err = worktree.Pull(&git.PullOptions{
			ReferenceName: plumbing.NewBranchReferenceName(branch),
			Auth:          auth,
		})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			log.Printf("Failed to pull latest changes: %v", err)
			return err
		}

		log.Println("Repository updated successfully.")
	}

	return nil
}
