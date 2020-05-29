Contributing to PassWall
=============================

You are here to help on PassWall? Awesome, feel welcome and read the
following sections in order to know how to ask questions and how to work on something.

Get in touch
------------

- Ask usage questions ("How do I?") on [StackOverflow](https://stackoverflow.com/questions/tagged/passwall).
- Report bugs or suggest features on [GitHub issues](https://github.com/pass-wall/passwall-server/issues).
- Discuss topics on [Slack](https://passwall.slack.com).
- Email us at [hello@passwall.io](mailto:hello@passwall.io).

How to find something to contribute?
------------

1. First look for [help wanted](https://github.com/pass-wall/passwall-server/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22) issues.

1. Then you can try to fix [// TODO:](https://github.com/pass-wall/passwall-server/search?q=TODO&unscoped_q=TODO)'s in the code.

1. If you have a good idea as a `feature` or find a `bug`, feel free to open an issue about it and tell us that you want to work on this subject.

Assignment
------------

When you find something to contribute;
1. Open an issue about it,

1. Make sure that nobody assigned for that issue,

1. Tell us that you want to work on the issue and get you assigned.

Commits and Pull Requests
------------

Good pull requests - patches, improvements, new features - are a fantastic help. They should remain focused in scope and avoid containing unrelated commits.

Please ask first before embarking on any significant pull request (e.g. implementing features, refactoring code), otherwise you risk spending a lot of time working on something that the project's developers might not want to merge into the project.

PassWall uses the branch naming policy below.

### Branch naming policy

<table>
  <thead>
    <tr>
      <th>Instance</th>
      <th>Branch</th>
      <th>Description, Instructions, Notes</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>Stable</td>
      <td>stable</td>
      <td>Accepts merges from Working and Hotfixes</td>
    </tr>
    <tr>
      <td>Working</td>
      <td>master</td>
      <td>Accepts merges from Features/Issues and Hotfixes</td>
    </tr>
    <tr>
      <td>Features/Issues</td>
      <td>topic-*</td>
      <td>Always branch off HEAD of Working</td>
    </tr>
    <tr>
      <td>Hotfix</td>
      <td>hotfix-*</td>
      <td>Always branch off Stable</td>
    </tr>
  </tbody>
</table>

More about branchs and workflow [here](https://gist.github.com/digitaljhelms/4287848)

### For new Contributors

If you never created a pull request before, welcome :tada: :smile: [Here is a great tutorial](https://egghead.io/series/how-to-contribute-to-an-open-source-project-on-github)
on how to send one :)

1. [Fork](http://help.github.com/fork-a-repo/) the project, clone your fork,
   and configure the remotes:

   ```bash
   # Clone your fork of the repo into the current directory
   git clone https://github.com/<your-username>/<repo-name>
   # Navigate to the newly cloned directory
   cd <repo-name>
   # Assign the original repo to a remote called "upstream"
   git remote add upstream https://github.com/pass-wall/<repo-name>
   ```

2. If you cloned a while ago, get the latest changes from upstream:

   ```bash
   git checkout master
   git pull upstream master
   ```

3. Create a new topic branch (off the main project development branch) to
   contain your feature, change, or fix:

   ```bash
   git checkout -b <topic-branch-name>
   ```

4. Make sure to update or add to the tests when appropriate. Patches and
   features will not be accepted without tests. 

5. If you added or changed a feature, make sure to document it accordingly in
   the `README.md` file.

6. Push your topic branch up to your fork:

   ```bash
   git push origin <topic-branch-name>
   ```

8. [Open a Pull Request](https://help.github.com/articles/using-pull-requests/)
    with a clear title and description.
    

How to report a bug
------------

When filing an issue, make sure to answer these five questions:
1. What version of Go are you using (go version)?
2. What operating system and processor architecture are you using?
3. What did you do?
4. What did you expect to see?
5. What did you see instead?
