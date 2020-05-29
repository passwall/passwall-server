PassWall'e Katkı
=============================

PassWall'e yardım etmek mi istiyorsun? Mükemmel, aramıza hoşgeldin. Bu dokümanı projeye nasıl katkıda bulunabileceğini göstermek için hazırladık. Gelecek katkıların için şimdiden teşekkürler.

İletişim kanalları
------------

- "Nasıl yapılır?" soruları için [StackOverflow](https://stackoverflow.com/questions/tagged/passwall).
- Hata (Bug) bildirimi, özellik (feature) önerisi veya proje kaynak kodu için [GitHub](https://github.com/pass-wall/passwall-server/issues).
- Konu tartışmaları için [Slack](https://passwall.slack.com).
- E-posta ile iletişim için [hello@passwall.io](mailto:hello@passwall.io).

Katkıda bulunacak bir şeyi nasıl bulabilirim ?
------------
1. Öncelikle katkıda bulunulacak her konunun bir issue'su olması gerektiğini unutmayın. Bunun için [issue](https://github.com/pass-wall/passwall-server/issues) sayfasına bakabilirsiniz.

1. Issue sayfasında öncelikle [help wanted](https://github.com/pass-wall/passwall-server/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22) issue'larına bakın.

1. Sonrasında koddaki  [// TODO:](https://github.com/pass-wall/passwall-server/search?q=TODO&unscoped_q=TODO)  kısımlarını düzeltmeyi deneyebilirsiniz.

1. Eğer yeni bir özellik (feature) olarak iyi bir fikriniz varsa veya bir hata (bug) bulursanız bu konuda bir konu (issue) açmaktan çekinmeyin ve eğer konu üzerinde çalışmak istiyorsanız mutlaka belirtin.

Görevlendirmeler
------------

Katkıda bulunucak bir şey bulduğunuzda;
1. Eğer henüz açılmamışsa onunla ilgili bir issue açın,

1. Bu issue için kimsenin görevlendirilmediğinden emin olun,

1. Issue üzerinde çalışmak istediğinizi açmış olduğunuz issue'nun sonunda belirtin.

Bu işlemler sonrasında ilgili issue için görevlendirilirsisniz (assign).

Commit'ler ve Pull Request'ler
------------

Nitelikli pull request'ler - yamalar, iyileştirmeler, yeni özellikler -  bizim için harika yardımlardır. Bu yamalar, iyileştirmeler, yeni özellikler için pull request'ler yapılırken konuya (issue) odaklanılmalı ve konu ile ilgilisi olmayan commit atmaktan kaçınılmalıdır.

Lütfen büyük kapsamlı ve ciddi pull request yapmadan önce bilgilendirme yapın (yeni özellikleri uygulama, kod düzenleme gibi). Aksi takdirde proje geliştiricilerinin değişiklik yapılmasını istemeyebileceği bir feature vb. üzerinde çalışmak için gereksiz zaman harcama riskiyle karşı karşıya kalabilirsiniz.

### Branch adlandırma politikası
PassWall aşağıdaki branch adlandırma politikasını kullanır.

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

Eğer daha önce hiç pull request yapmadıysanız aramıza hoşgeldiniz :tada: :smile:. 

1.  Projeyi öncelikle fork'layın yani kendi alanınıza alın. ([Fork](http://help.github.com/fork-a-repo/)) ve remote'ları yapılandırın:
   
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

3. Feature, fix ve değişiklikleriniz için yeni bir branch oluşturun (ana projenin development branch'ı olan master dışında):
   
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
    
Açık kaynak projeye nasıl katkıda bulunulabileceğini anlatan daha detaylı bir dokümana [şuradan](https://egghead.io/series/how-to-contribute-to-an-open-source-project-on-github) ulaşabilirsiniz.

Hata bildirimi (Bug report)
------------

Bir hata bildirimi için issue açarken aşağıdaki beş soruya cevap verdiğinizden emin olun.
1. Kullanılan GO sürümü nedir?
2. Hangi işletim sistemi ve işlemci mimarisi kullanıyorsunuz?
3. Ne yaptın?
4. Ne görmeyi bekliyordun?
5. Onun yerine ne gördün?
