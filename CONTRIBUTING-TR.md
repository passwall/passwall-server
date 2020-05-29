PassWall'e Katkı
=============================

PassWall'e yardım etmek mi istiyorsun? Mükemmel, aramıza hoşgeldin. Aşağıda nasıl soru sorulması gerektiği ve bir şey üzerinde nasıl çalışman gerektiği yazıyor.

Lütfen tüm mecralarda konuksever ve arkadaş canlısı olmayı unutmayın.

Temasta olmak
------------

-  "Nasıl yapılır?" soruları için [StackOverflow](https://stackoverflow.com/questions/tagged/passwall) .
- Bug report, feature önerisi veya proje kaynak kodu için [GitHub](https://github.com/pass-wall/passwall-server/issues).
- Konu tartışmaları için [Slack](https://passwall.slack.com).
- E-posta gönderin [hello@passwall.io](mailto:hello@passwall.io).

Katkıda bulunacak bir şey nasıl bulabilirim ?
------------

1. İlk önce [help wanted](https://github.com/pass-wall/passwall-server/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22) issue'larına bakın.

1. Sonrasında koddaki  [// TODO:](https://github.com/pass-wall/passwall-server/search?q=TODO&unscoped_q=TODO)  kısımlarını düzeltmeyi deneyebilirsiniz.

1. Eğer bir "feature" olarak iyi bir fikriniz varsa veya bir "bug" bulursanız bu konuda bir issue açmaktan çekinmeyin ve konuda hakkında bizimle çalışmak istediğinizi bildirin.

Görevlendirmeler
------------

Katkıda bulunucak bir şey bulduğunuzda;
1. Onunla ilgili bir issue açın,

1. Bu issue için kimsenin görevlendirilmediğinden emin olun,

1. Issue üzerinde sizin çalışmak istediğinizi bize bildirin.

Commits ve Pull Requests
------------

Nitelikli pull requests - yamalar, iyileştirmeler, yeni özellikler  bizim için harika yardımlardır. Bu yamalar, iyileştirmeler, yeni özellikler için pull request'ler yapılırken kapsam içine odaklanılmalı ve alakalı olmayan commit'ler kullanmaktan kaçınılmalıdır.

Lütfen pull request yapmadan önce bilgilendirme yapın (yeni özellikleri uygulama, kod düzenleme gibi). Aksi takdirde proje geliştiricilerinin değişiklik yapılmasını istemeyebileceği bir feature vb. üzerinde çalışmak için gereksiz zaman harcama riskiyle karşı karşıya kalabilirsiniz.

PassWall aşağıdaki branch adlandırma politikasını kullanır.

### Branch adlandırma politikası

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

Branch ve workflow hakkında daha fazla bilgi [burada](https://gist.github.com/digitaljhelms/4287848)

### Yeni Contributor'lar için

Eğer daha önce hiç pull request yapmadıysanız aramıza hoşgeldiniz. İşte nasıl pull request yapabileceğiniz üzerine :tada: :smile: [Harika bir öğretici](https://egghead.io/series/how-to-contribute-to-an-open-source-project-on-github)

1.  Proje fork'unuzu alın ([Fork](http://help.github.com/fork-a-repo/)) ve remote'ları yapılandırın:
   
```bash
   # Repo forkunuzu geçerli dizin üzerine klonlayın
   git clone https://github.com/<your-username>/<repo-name>
   # Klonlanan dizine gidin
   cd <repo-name>
   # Orjinal repoyu "upstream" adlı bir remote called'a atayın
   git remote add upstream https://github.com/hoodiehq/<repo-name>
   ```
   
2. Eğer daha önce fork yaptıysanız, upstream üzerinden en son değişiklikleri alın:

```bash
   git checkout master
   git pull upstream master
```

3. Feature, fix ve değişiklikleriniz için yeni bir branch oluşturun (ana projenin development branch'ı dışında):
   
```bash
   git checkout -b <topic-branch-name>
   ```
   
4. Uygun olduğunda testleri güncellediğinizden veya yeni bir test eklediğinizden emin olun. Patch'ler ve feature'lar test olmadan kabul edilmeyecektir. 
   
5. Eklediğiniz veya değişiklik yaptığınız düzenlemelerin belgelendirmesini  `README.md` dosyası üzerinde yapmayı unutmayın.
   
6. Kendi oluşturduğunuz branch'ınız üzerinden fork'unuza push edin:

```bash
   git push origin <topic-branch-name>
```

7. Net, anlaşılır bir başlık ve açıklama ile pull request açın. [Konu hakkında yardımcı döküman](https://help.github.com/articles/using-pull-requests/)
    

Bug Report
------------

Bir issue yazarken aşağıdaki beş soruya cevap verdiğinizden emin olun.
1. Kullanılan GO sürümü nedir?
2. Hangi işletim sistemi ve işlemci mimarisi kullanıyorsunuz?
3. Ne yaptın?
4. Ne görmeyi bekliyordun?
5. Onun yerine ne gördün?

Son olarak kopyala-yapıştır için bir şablon ekleyebilirsiniz ve böylece işler daha da kolaylaşabilir. 