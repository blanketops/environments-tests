type GitRepository struct{
   Provider string
   HookUrl string
   Repository Repository
   Webhooks Webooks
}

type Repository {
  Owner string
  Name string
}

type Webhooks {
  Events[] Events
}

type Events{
  Event string
}
