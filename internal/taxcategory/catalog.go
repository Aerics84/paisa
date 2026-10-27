package taxcategory

// CapitalIncomeSection is generated at report time and is not taggable.
const CapitalIncomeSection = "capital_income"

type Section struct {
	Key   string
	Label string
	Form  string
}

type Category struct {
	Key     string
	Label   string
	Section string
}

var Sections = []Section{
	{Key: "work_expenses", Label: "Werbungskosten", Form: "Anlage N"},
	{Key: "insurance", Label: "Vorsorgeaufwand", Form: "Anlage Vorsorgeaufwand"},
	{Key: "extraordinary", Label: "Außergewöhnliche Belastungen", Form: "Hauptvordruck"},
	{Key: "household", Label: "Haushaltsnahe Dienstleistungen", Form: "Hauptvordruck"},
	{Key: "donations", Label: "Spenden und Mitgliedsbeiträge", Form: "Hauptvordruck"},
	{Key: CapitalIncomeSection, Label: "Kapitalerträge", Form: "Anlage KAP"},
}

var Categories = []Category{
	{Key: "work_equipment", Label: "Arbeitsmittel", Section: "work_expenses"},
	{Key: "work_training", Label: "Fortbildung und Fachliteratur", Section: "work_expenses"},
	{Key: "work_travel", Label: "Reisekosten", Section: "work_expenses"},
	{Key: "work_commute", Label: "Wege zur Arbeit", Section: "work_expenses"},
	{Key: "work_home_office", Label: "Homeoffice", Section: "work_expenses"},
	{Key: "work_phone_internet", Label: "Telefon und Internet", Section: "work_expenses"},
	{Key: "tax_advice", Label: "Steuerberatung und -software", Section: "work_expenses"},
	{Key: "work_other", Label: "Sonstige Werbungskosten", Section: "work_expenses"},
	{Key: "health_insurance", Label: "Kranken- und Pflegeversicherung", Section: "insurance"},
	{Key: "other_insurance", Label: "Sonstige Versicherungen", Section: "insurance"},
	{Key: "health", Label: "Krankheitskosten", Section: "extraordinary"},
	{Key: "household_service", Label: "Haushaltsnahe Dienstleistungen", Section: "household"},
	{Key: "craftsman", Label: "Handwerkerleistungen", Section: "household"},
	{Key: "donation", Label: "Spenden und Mitgliedsbeiträge", Section: "donations"},
}

func IsKnownCategory(key string) bool {
	_, ok := CategoryByKey(key)
	return ok
}

func CategoryByKey(key string) (Category, bool) {
	for _, category := range Categories {
		if category.Key == key {
			return category, true
		}
	}
	return Category{}, false
}

func CategoriesForSection(section string) []Category {
	matched := []Category{}
	for _, category := range Categories {
		if category.Section == section {
			matched = append(matched, category)
		}
	}
	return matched
}

func CategoryKeys() []string {
	keys := make([]string, 0, len(Categories))
	for _, category := range Categories {
		keys = append(keys, category.Key)
	}
	return keys
}
